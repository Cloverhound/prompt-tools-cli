package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
	"github.com/Cloverhound/prompt-tools-cli/internal/gcpauth"
	"github.com/Cloverhound/prompt-tools-cli/internal/keyring"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard (first-run experience)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetup()
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// hasProviderKey returns true if an API key is configured (env var or keyring) for the provider.
func hasProviderKey(name string) bool {
	envVars := map[string]string{
		"google":     "GOOGLE_API_KEY",
		"elevenlabs": "ELEVENLABS_API_KEY",
		"assemblyai": "ASSEMBLYAI_API_KEY",
		"openai":     "OPENAI_API_KEY",
	}
	if envVar, ok := envVars[name]; ok {
		if os.Getenv(envVar) != "" {
			return true
		}
	}
	if _, err := keyring.GetAPIKey(name); err == nil {
		return true
	}
	return false
}

// hasGoogleAuth returns true if Google has any auth configured (ADC, env var, or keyring).
func hasGoogleAuth() bool {
	return gcpauth.HasCredentials() || hasProviderKey("google")
}

func runSetup() error {
	fmt.Print("\n  Welcome to Prompt Tools! Let's get you set up.\n\n")

	// Detect which providers already have credentials configured
	ttsConfigured := map[string]bool{
		"google":     hasGoogleAuth(),
		"elevenlabs": hasProviderKey("elevenlabs"),
		"openai":     hasProviderKey("openai"),
	}
	sttConfigured := map[string]bool{
		"google":     hasGoogleAuth(),
		"assemblyai": hasProviderKey("assemblyai"),
		"openai":     hasProviderKey("openai"),
	}

	// Step 1: Select TTS providers (preselect already-configured ones)
	var ttsProviders []string
	ttsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which text-to-speech providers would you like to use?").
				Options(
					huh.NewOption("Google Cloud TTS  (400+ voices, native IVR formats, SSML support)", "google").
						Selected(ttsConfigured["google"]),
					huh.NewOption("ElevenLabs        (Premium natural voices, requires format conversion)", "elevenlabs").
						Selected(ttsConfigured["elevenlabs"]),
					huh.NewOption("OpenAI            (High quality natural voices, simple API)", "openai").
						Selected(ttsConfigured["openai"]),
				).
				Value(&ttsProviders),
		),
	)
	if err := ttsForm.Run(); err != nil {
		return err
	}
	if len(ttsProviders) == 0 {
		return fmt.Errorf("at least one TTS provider must be selected")
	}

	// Step 2: Select STT providers (preselect already-configured ones)
	var sttProviders []string
	sttForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which speech-to-text providers would you like to use?").
				Options(
					huh.NewOption("Google Cloud STT  (Phrase boosting, word timestamps, long audio support)", "google").
						Selected(sttConfigured["google"]),
					huh.NewOption("AssemblyAI        (High accuracy, simple API, automatic punctuation)", "assemblyai").
						Selected(sttConfigured["assemblyai"]),
					huh.NewOption("OpenAI            (Whisper & GPT-4o transcription, sync API)", "openai").
						Selected(sttConfigured["openai"]),
				).
				Value(&sttProviders),
		),
	)
	if err := sttForm.Run(); err != nil {
		return err
	}
	if len(sttProviders) == 0 {
		return fmt.Errorf("at least one STT provider must be selected")
	}

	// Step 3: Collect API keys for selected providers
	needsKey := make(map[string]bool)
	for _, p := range ttsProviders {
		needsKey[p] = true
	}
	for _, p := range sttProviders {
		needsKey[p] = true
	}

	// Google — offer OAuth2/ADC or API key
	if needsKey["google"] {
		if err := setupGoogle(); err != nil {
			return err
		}
	}

	// ElevenLabs key
	if needsKey["elevenlabs"] {
		if err := setupProviderKey("ElevenLabs", "elevenlabs"); err != nil {
			return err
		}
	}

	// AssemblyAI key
	if needsKey["assemblyai"] {
		if err := setupProviderKey("AssemblyAI", "assemblyai"); err != nil {
			return err
		}
	}

	// OpenAI key
	if needsKey["openai"] {
		if err := setupProviderKey("OpenAI", "openai"); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "\n  ✓ Setup complete\n")
	fmt.Fprintf(os.Stderr, "\n  You're all set! Try: prompt-tools speak \"Hello world\" --output hello.wav\n")
	fmt.Fprintf(os.Stderr, "  Customize defaults with: prompt-tools config set-provider, set-voice, set-format, etc.\n\n")

	return nil
}

// setupProviderKey handles API key setup for a non-Google provider.
// If a key already exists, offers to keep or replace it.
func setupProviderKey(displayName, providerName string) error {
	fmt.Printf("\n  --- %s ---\n", displayName)

	if hasProviderKey(providerName) {
		// Key already configured — offer to keep or change
		var action string
		actionForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("%s API key is already configured", displayName)).
					Options(
						huh.NewOption("Keep existing key", "keep"),
						huh.NewOption("Enter a new key", "change"),
					).
					Value(&action),
			),
		)
		if err := actionForm.Run(); err != nil {
			return err
		}
		if action == "keep" {
			fmt.Println("  ✓ Keeping existing API key")
			return nil
		}
	}

	var apiKey string
	keyForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("%s API Key", displayName)).
				EchoMode(huh.EchoModePassword).
				Value(&apiKey),
		),
	)
	if err := keyForm.Run(); err != nil {
		return err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey != "" {
		if err := keyring.SetAPIKey(providerName, apiKey); err != nil {
			return fmt.Errorf("saving %s API key: %w", displayName, err)
		}
		fmt.Println("  ✓ API key saved to system keyring")
	}
	return nil
}

// gcloudInstaller returns the package manager name, display label, and install command
// for the current platform, or empty strings if no supported package manager is found.
func gcloudInstaller() (name string, label string, cmd []string) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			return "brew", "brew install google-cloud-sdk", []string{"brew", "install", "--cask", "google-cloud-sdk"}
		}
	case "linux":
		if _, err := exec.LookPath("snap"); err == nil {
			return "snap", "snap install google-cloud-cli --classic", []string{"snap", "install", "google-cloud-cli", "--classic"}
		}
		if _, err := exec.LookPath("apt-get"); err == nil {
			return "apt", "apt-get install google-cloud-cli", []string{"apt-get", "install", "-y", "google-cloud-cli"}
		}
	case "windows":
		if _, err := exec.LookPath("winget"); err == nil {
			return "winget", "winget install Google.CloudSDK", []string{"winget", "install", "Google.CloudSDK"}
		}
		if _, err := exec.LookPath("choco"); err == nil {
			return "choco", "choco install gcloudsdk", []string{"choco", "install", "gcloudsdk", "-y"}
		}
	}
	return "", "", nil
}

// offerGcloudInstall detects a package manager and offers to install gcloud CLI.
// Returns true if gcloud was successfully installed.
func offerGcloudInstall() bool {
	_, label, installArgs := gcloudInstaller()
	if installArgs == nil {
		fmt.Println("  gcloud CLI not found. Install it for OAuth2 auth (enables --style voice steering):")
		fmt.Println("  https://cloud.google.com/sdk/docs/install")
		fmt.Println()
		return false
	}

	var action string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("gcloud CLI not found — required for OAuth2 auth (--style voice steering)").
				Options(
					huh.NewOption(fmt.Sprintf("Install now (%s)", label), "install"),
					huh.NewOption("Skip — use API key instead", "skip"),
				).
				Value(&action),
		),
	)
	if err := form.Run(); err != nil || action == "skip" {
		return false
	}

	fmt.Printf("  Installing gcloud CLI via %s...\n", label)
	installCmd := exec.Command(installArgs[0], installArgs[1:]...)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ Installation failed: %v\n", err)
		fmt.Println("  Install manually: https://cloud.google.com/sdk/docs/install")
		fmt.Println()
		return false
	}

	fmt.Println("  ✓ gcloud CLI installed")
	return true
}

// setupGoogleNoGcloud handles Google setup when gcloud is not available — API key or skip.
func setupGoogleNoGcloud() error {
	var action string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Google authentication").
				Options(
					huh.NewOption("Enter API key", "apikey"),
					huh.NewOption("Skip — configure Google later", "skip"),
				).
				Value(&action),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	if action == "skip" {
		fmt.Println("  Skipped — configure later with: prompt-tools config set-api-key google")
		return nil
	}
	return setupGoogleAPIKey()
}

// setupGoogle handles Google auth setup — offers OAuth2/ADC or API key depending on gcloud availability.
func setupGoogle() error {
	fmt.Println("\n  --- Google Cloud TTS & STT ---")

	_, gcloudErr := exec.LookPath("gcloud")
	hasGcloud := gcloudErr == nil

	if !hasGcloud {
		// gcloud not found — offer to install if package manager available
		if installed := offerGcloudInstall(); installed {
			// Re-check after install
			_, gcloudErr = exec.LookPath("gcloud")
			hasGcloud = gcloudErr == nil
		}
		if !hasGcloud {
			return setupGoogleNoGcloud()
		}
	}

	// Detect existing auth state
	hasADC := gcpauth.HasCredentials()
	hasAPIKey := hasProviderKey("google")

	if hasADC || hasAPIKey {
		// Show what's already configured
		if hasADC {
			fmt.Println("  ✓ GCP Application Default Credentials found")
		}
		if hasAPIKey {
			fmt.Println("  ✓ Google API key configured")
		}

		var action string
		actionForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Google authentication").
					Options(
						huh.NewOption("Keep existing configuration", "keep"),
						huh.NewOption("Reconfigure with GCP OAuth2", "oauth2"),
						huh.NewOption("Reconfigure with API key", "apikey"),
					).
					Value(&action),
			),
		)
		if err := actionForm.Run(); err != nil {
			return err
		}

		switch action {
		case "keep":
			fmt.Println("  ✓ Keeping existing Google auth")
			// Still offer project setup if using ADC
			if hasADC {
				return setupGCPProject()
			}
			return nil
		case "apikey":
			return setupGoogleAPIKey()
		default: // oauth2
			return setupGoogleOAuth2()
		}
	}

	// Nothing configured — offer choice
	var authMethod string
	authForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Google authentication method").
				Options(
					huh.NewOption("GCP OAuth2 (recommended) — enables --style voice steering", "oauth2"),
					huh.NewOption("API Key — simpler, fewer features", "apikey"),
					huh.NewOption("Skip — configure Google later", "skip"),
				).
				Value(&authMethod),
		),
	)
	if err := authForm.Run(); err != nil {
		return err
	}

	switch authMethod {
	case "skip":
		fmt.Println("  Skipped — configure later with: prompt-tools setup")
		return nil
	case "apikey":
		return setupGoogleAPIKey()
	default:
		return setupGoogleOAuth2()
	}
}

// setupGoogleAPIKey handles the API key flow.
func setupGoogleAPIKey() error {
	if hasProviderKey("google") {
		var action string
		actionForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Google API key is already configured").
					Options(
						huh.NewOption("Keep existing key", "keep"),
						huh.NewOption("Enter a new key", "change"),
					).
					Value(&action),
			),
		)
		if err := actionForm.Run(); err != nil {
			return err
		}
		if action == "keep" {
			fmt.Println("  ✓ Keeping existing API key")
			return nil
		}
	}

	var googleKey string
	keyForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Google Cloud API Key").
				EchoMode(huh.EchoModePassword).
				Value(&googleKey),
		),
	)
	if err := keyForm.Run(); err != nil {
		return err
	}
	googleKey = strings.TrimSpace(googleKey)
	if googleKey != "" {
		if err := keyring.SetAPIKey("google", googleKey); err != nil {
			return fmt.Errorf("saving Google API key: %w", err)
		}
		fmt.Println("  ✓ API key saved to system keyring")
	}
	return nil
}

// setupGoogleOAuth2 handles the OAuth2/ADC flow.
func setupGoogleOAuth2() error {
	// Check if ADC credentials already exist
	if gcpauth.HasCredentials() {
		fmt.Println("  ✓ Application Default Credentials found")
		// Verify they work
		token, err := gcpauth.GetToken("https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ Credentials found but token exchange failed: %v\n", err)
			fmt.Println("  Try running: gcloud auth application-default login")
			return nil
		}
		if token != nil {
			fmt.Println("  ✓ Credentials verified — token exchange successful")
		}

		return setupGCPProject()
	}

	// No ADC credentials — offer to run gcloud auth
	fmt.Println("  No Application Default Credentials found.")
	var runLogin bool
	loginForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Run 'gcloud auth application-default login' now?").
				Value(&runLogin),
		),
	)
	if err := loginForm.Run(); err != nil {
		return err
	}

	if !runLogin {
		fmt.Println("  Skipped — run manually later: gcloud auth application-default login")
		// Fall back to API key
		return setupGoogleAPIKey()
	}

	// Run gcloud auth with passthrough stdin/stdout
	loginCmd := exec.Command("gcloud", "auth", "application-default", "login")
	loginCmd.Stdin = os.Stdin
	loginCmd.Stdout = os.Stdout
	loginCmd.Stderr = os.Stderr
	if err := loginCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ gcloud auth failed: %v\n", err)
		fmt.Println("  Falling back to API key setup")
		return setupGoogleAPIKey()
	}

	// Verify the new credentials
	token, err := gcpauth.GetToken("https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ Token verification failed: %v\n", err)
		return nil
	}
	if token != nil {
		fmt.Println("  ✓ Credentials verified — GCP OAuth2 authentication ready")
	}

	return setupGCPProject()
}

// setupGCPProject detects the gcloud default project and lets the user confirm or override it.
func setupGCPProject() error {
	// Check if project is already configured in prompt-tools
	cfg, err := appconfig.Load()
	if err != nil {
		return err
	}
	existingProject := ""
	if gc, ok := cfg.Providers["google"]; ok {
		existingProject = gc.ProjectID
	}

	// Detect default project from gcloud
	var gcloudProject string
	if out, err := exec.Command("gcloud", "config", "get-value", "project").Output(); err == nil {
		gcloudProject = strings.TrimSpace(string(out))
	}

	// Build options based on what's available
	var options []huh.Option[string]

	if existingProject != "" {
		options = append(options, huh.NewOption(fmt.Sprintf("Keep current: %s", existingProject), "existing"))
	}
	if gcloudProject != "" && gcloudProject != existingProject {
		options = append(options, huh.NewOption(fmt.Sprintf("Use gcloud default: %s", gcloudProject), "gcloud"))
	}
	options = append(options, huh.NewOption("Enter a different project ID", "custom"))

	var choice string
	projectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("GCP project for prompt-tools").
				Options(options...).
				Value(&choice),
		),
	)
	if err := projectForm.Run(); err != nil {
		return err
	}

	var projectID string
	switch choice {
	case "existing":
		fmt.Printf("  ✓ GCP project: %s\n", existingProject)
		return nil
	case "gcloud":
		projectID = gcloudProject
	case "custom":
		inputForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("GCP Project ID").
					Value(&projectID),
			),
		)
		if err := inputForm.Run(); err != nil {
			return err
		}
		projectID = strings.TrimSpace(projectID)
	}

	if projectID != "" {
		gc := cfg.Providers["google"]
		gc.ProjectID = projectID
		cfg.Providers["google"] = gc
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("  ✓ GCP project: %s\n", projectID)
	}

	return nil
}
