package tts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/audio"
	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
)

const elevenlabsEndpoint = "https://api.elevenlabs.io/v1"

type ElevenLabsTTS struct {
	apiKey string
}

func NewElevenLabsTTS(auth provider.AuthConfig) provider.TTSProvider {
	return &ElevenLabsTTS{apiKey: auth.APIKey}
}

func init() {
	provider.RegisterTTS("elevenlabs", func(auth provider.AuthConfig) provider.TTSProvider {
		return NewElevenLabsTTS(auth)
	})
}

func (e *ElevenLabsTTS) Name() string { return "elevenlabs" }

func (e *ElevenLabsTTS) Synthesize(req *provider.TTSRequest) (*provider.TTSResult, error) {
	// Resolve voice: accept either a voice ID or a friendly name
	voiceID, err := e.resolveVoiceID(req.Voice)
	if err != nil {
		return nil, err
	}

	text := req.Text
	if req.SSML != "" {
		// ElevenLabs doesn't support SSML, strip tags and use plain text
		text = req.SSML
	}

	// Request PCM output from ElevenLabs so we can convert to mulaw/alaw
	outputFormat := "pcm_16000" // 16-bit PCM at 16kHz
	if req.Encoding == audio.EncodingMP3 {
		outputFormat = "mp3_44100_128"
	}

	// Default to eleven_v3; allow override via --model
	modelID := "eleven_v3"
	if req.Model != "" {
		modelID = req.Model
	}

	body := map[string]any{
		"text":     text,
		"model_id": modelID,
		"voice_settings": map[string]any{
			"stability":        0.5,
			"similarity_boost": 0.75,
		},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/text-to-speech/%s?output_format=%s", elevenlabsEndpoint, voiceID, outputFormat)

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s\n", url)
	}

	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("xi-api-key", e.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ElevenLabs TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ElevenLabs API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// If MP3 was requested, return as-is
	if req.Encoding == audio.EncodingMP3 {
		return &provider.TTSResult{
			AudioData:  respBody,
			Format:     audio.EncodingMP3,
			SampleRate: 44100,
		}, nil
	}

	// PCM data from ElevenLabs is 16-bit at 16kHz
	pcmData := respBody
	srcRate := 16000

	// Resample if target rate differs
	if req.SampleRate != srcRate {
		pcmData = audio.ResampleLinear16(pcmData, srcRate, req.SampleRate)
	}

	// Convert to target encoding
	var audioData []byte
	format := req.Encoding
	switch req.Encoding {
	case audio.EncodingMulaw:
		audioData = audio.Linear16ToMulaw(pcmData)
	case audio.EncodingAlaw:
		audioData = audio.Linear16ToAlaw(pcmData)
	case audio.EncodingLinear16:
		audioData = pcmData
	default:
		audioData = audio.Linear16ToMulaw(pcmData)
		format = audio.EncodingMulaw
	}

	return &provider.TTSResult{
		AudioData:  audioData,
		Format:     format,
		SampleRate: req.SampleRate,
	}, nil
}

// resolveVoiceID resolves a voice name or ID to an ElevenLabs voice ID.
// If the input matches a voice ID directly, it's returned as-is.
// Otherwise, it searches by name (case-insensitive).
func (e *ElevenLabsTTS) resolveVoiceID(voice string) (string, error) {
	// ElevenLabs voice IDs are 20-char alphanumeric strings.
	// If it looks like one, use it directly.
	if len(voice) == 20 && isAlphanumeric(voice) {
		return voice, nil
	}

	// Look up by friendly name (match full name or short name before " - ")
	voices, err := e.ListVoices("")
	if err != nil {
		return "", fmt.Errorf("resolving voice name %q: %w", voice, err)
	}
	for _, v := range voices {
		if strings.EqualFold(v.Name, voice) {
			return v.VoiceID, nil
		}
		// Match short name (e.g., "Sarah" matches "Sarah - Mature, Reassuring")
		if short, _, ok := strings.Cut(v.Name, " - "); ok && strings.EqualFold(short, voice) {
			return v.VoiceID, nil
		}
	}
	return "", fmt.Errorf("ElevenLabs voice %q not found — use 'prompt-tools voices --provider elevenlabs' to list available voices", voice)
}

func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func (e *ElevenLabsTTS) ListVoices(languageCode string) ([]provider.Voice, error) {
	url := fmt.Sprintf("%s/voices", elevenlabsEndpoint)

	if config.Debug() {
		fmt.Printf("[DEBUG] GET %s\n", url)
	}

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("xi-api-key", e.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("listing ElevenLabs voices: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ElevenLabs API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Voices []struct {
			VoiceID string            `json:"voice_id"`
			Name    string            `json:"name"`
			Labels  map[string]string `json:"labels"`
		} `json:"voices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing voices: %w", err)
	}

	var voices []provider.Voice
	for _, v := range result.Voices {
		gender := ""
		if g, ok := v.Labels["gender"]; ok {
			gender = strings.ToUpper(g)
		}

		voice := provider.Voice{
			Name:          v.Name,
			VoiceID:       v.VoiceID,
			Model:         "ElevenLabs",
			LanguageCodes: []string{"multilingual"},
			Gender:        gender,
			Provider:      "elevenlabs",
		}

		voices = append(voices, voice)
	}

	return voices, nil
}
