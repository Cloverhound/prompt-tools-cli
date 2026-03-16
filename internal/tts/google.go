package tts

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"

	"github.com/Cloverhound/prompt-tools-cli/internal/audio"
	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
)

const googleTTSEndpoint = "https://texttospeech.googleapis.com/v1"

type GoogleTTS struct {
	apiKey string
}

func NewGoogleTTS(apiKey string) provider.TTSProvider {
	return &GoogleTTS{apiKey: apiKey}
}

func init() {
	provider.RegisterTTS("google", func(apiKey string) provider.TTSProvider {
		return NewGoogleTTS(apiKey)
	})
}

func (g *GoogleTTS) Name() string { return "google" }

// isGeminiVoice returns true if the voice name is a bare Gemini TTS voice
// (single capitalized word like "Achernar") rather than a structured name.
func isGeminiVoice(name string) bool {
	if strings.Contains(name, "-") {
		return false
	}
	if len(name) == 0 {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

func (g *GoogleTTS) Synthesize(req *provider.TTSRequest) (*provider.TTSResult, error) {
	// Build request body
	var input map[string]string
	if req.SSML != "" {
		input = map[string]string{"ssml": req.SSML}
	} else {
		input = map[string]string{"text": req.Text}
	}

	// Gemini voices require model_name and use "en-US" as language code
	gemini := isGeminiVoice(req.Voice)

	langCode := "en-US"
	if !gemini {
		parts := strings.SplitN(req.Voice, "-", 3)
		if len(parts) >= 2 {
			langCode = parts[0] + "-" + parts[1]
		}
	}

	voice := map[string]string{
		"languageCode": langCode,
		"name":         req.Voice,
	}

	audioConfig := map[string]any{
		"audioEncoding":   audio.GoogleEncoding(req.Encoding),
		"sampleRateHertz": req.SampleRate,
	}
	if req.SpeakingRate != 0 {
		audioConfig["speakingRate"] = req.SpeakingRate
	}
	if req.Pitch != 0 {
		audioConfig["pitch"] = req.Pitch
	}
	if req.VolumeGainDb != 0 {
		audioConfig["volumeGainDb"] = req.VolumeGainDb
	}

	body := map[string]any{
		"input":       input,
		"voice":       voice,
		"audioConfig": audioConfig,
	}

	// Gemini voices require a model_name parameter
	if gemini || req.Model != "" {
		model := req.Model
		if model == "" {
			model = "gemini-2.5-pro-tts"
		}
		body["model_name"] = model
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s/text:synthesize\n", googleTTSEndpoint)
		fmt.Printf("[DEBUG] Body: %s\n", string(bodyJSON))
	}

	url := fmt.Sprintf("%s/text:synthesize?key=%s", googleTTSEndpoint, g.apiKey)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, fmt.Errorf("Google TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google TTS API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AudioContent string `json:"audioContent"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	audioData, err := base64.StdEncoding.DecodeString(result.AudioContent)
	if err != nil {
		return nil, fmt.Errorf("decoding audio: %w", err)
	}

	return &provider.TTSResult{
		AudioData:  audioData,
		Format:     req.Encoding,
		SampleRate: req.SampleRate,
	}, nil
}

func (g *GoogleTTS) ListVoices(languageCode string) ([]provider.Voice, error) {
	url := fmt.Sprintf("%s/voices?key=%s", googleTTSEndpoint, g.apiKey)
	if languageCode != "" {
		url += "&languageCode=" + languageCode
	}

	if config.Debug() {
		fmt.Printf("[DEBUG] GET %s\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("listing voices: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google TTS API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Voices []struct {
			LanguageCodes     []string `json:"languageCodes"`
			Name              string   `json:"name"`
			SsmlGender        string   `json:"ssmlGender"`
			NaturalSampleRate int      `json:"naturalSampleRateHertz"`
		} `json:"voices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing voices: %w", err)
	}

	voices := make([]provider.Voice, len(result.Voices))
	for i, v := range result.Voices {
		voices[i] = provider.Voice{
			Name:              v.Name,
			Model:             parseGoogleModel(v.Name),
			LanguageCodes:     v.LanguageCodes,
			Gender:            v.SsmlGender,
			Provider:          "google",
			NaturalSampleRate: v.NaturalSampleRate,
		}
	}

	return voices, nil
}

// parseGoogleModel extracts the model type from a Google TTS voice name.
// The API provides no model metadata — we infer from the name pattern.
//
//	"en-US-Neural2-F"             → "Neural2"
//	"en-US-Chirp3-HD-Achernar"    → "Chirp3-HD"
//	"en-US-Chirp-HD-D"            → "Chirp-HD"
//	"Achernar"                    → "Gemini" (bare names are Gemini TTS voices)
func parseGoogleModel(name string) string {
	if isGeminiVoice(name) {
		return "Gemini"
	}
	parts := strings.Split(name, "-")
	// Structured: lang-region-Model[-qualifier][-variant]
	if len(parts) >= 3 {
		model := parts[2]
		// Check for HD qualifier (Chirp-HD, Chirp3-HD)
		if len(parts) >= 4 && parts[3] == "HD" {
			model += "-HD"
		}
		return model
	}
	return ""
}
