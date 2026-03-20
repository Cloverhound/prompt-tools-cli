package gcpauth

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const tokenEndpoint = "https://oauth2.googleapis.com/token"

// Token represents an OAuth2 access token.
type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

// GetToken resolves ADC credentials and exchanges them for an access token.
// Returns (nil, nil) if no credentials are found (caller should fall back to API key).
func GetToken(scopes ...string) (*Token, error) {
	// 1. Check GOOGLE_APPLICATION_CREDENTIALS env var
	if path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); path != "" {
		return tokenFromFile(path, scopes)
	}

	// 2. Check well-known ADC path
	adcPath := WellKnownADCPath()
	if _, err := os.Stat(adcPath); err == nil {
		return tokenFromFile(adcPath, scopes)
	}

	// No credentials found — not an error, caller falls back to API key
	return nil, nil
}

// HasCredentials returns true if ADC credential files exist.
func HasCredentials() bool {
	if path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	if _, err := os.Stat(WellKnownADCPath()); err == nil {
		return true
	}
	return false
}

// WellKnownADCPath returns the platform-specific ADC credential file path.
func WellKnownADCPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "gcloud", "application_default_credentials.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
}

// tokenFromFile reads a credential JSON file and exchanges it for a token.
func tokenFromFile(path string, scopes []string) (*Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}

	var cred struct {
		Type         string `json:"type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(data, &cred); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}

	switch cred.Type {
	case "authorized_user":
		return exchangeUserCredentials(cred.ClientID, cred.ClientSecret, cred.RefreshToken)
	case "service_account":
		return exchangeServiceAccountJWT(data, scopes)
	default:
		return nil, fmt.Errorf("unsupported credential type: %s", cred.Type)
	}
}

// exchangeUserCredentials exchanges a refresh token for an access token.
func exchangeUserCredentials(clientID, clientSecret, refreshToken string) (*Token, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
	}

	resp, err := http.PostForm(tokenEndpoint, form)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &Token{
		AccessToken: tokenResp.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

// exchangeServiceAccountJWT builds a JWT assertion and exchanges it for an access token.
func exchangeServiceAccountJWT(saJSON []byte, scopes []string) (*Token, error) {
	var sa struct {
		ClientEmail  string `json:"client_email"`
		PrivateKey   string `json:"private_key"`
		TokenURI     string `json:"token_uri"`
	}
	if err := json.Unmarshal(saJSON, &sa); err != nil {
		return nil, fmt.Errorf("parsing service account: %w", err)
	}

	endpoint := sa.TokenURI
	if endpoint == "" {
		endpoint = tokenEndpoint
	}

	// Parse RSA private key
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from service account key")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}

	// Build JWT
	now := time.Now()
	header := base64URLEncode(mustJSON(map[string]string{"alg": "RS256", "typ": "JWT"}))
	claims := base64URLEncode(mustJSON(map[string]any{
		"iss":   sa.ClientEmail,
		"scope": strings.Join(scopes, " "),
		"aud":   endpoint,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}))

	signingInput := header + "." + claims
	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(nil, rsaKey, 0, h[:])
	if err != nil {
		return nil, fmt.Errorf("signing JWT: %w", err)
	}

	jwt := signingInput + "." + base64URLEncode(sig)

	// Exchange JWT for access token
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwt},
	}

	resp, err := http.PostForm(endpoint, form)
	if err != nil {
		return nil, fmt.Errorf("JWT token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWT token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &Token{
		AccessToken: tokenResp.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
