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

func NewElevenLabsTTS(apiKey string) provider.TTSProvider {
	return &ElevenLabsTTS{apiKey: apiKey}
}

func init() {
	provider.RegisterTTS("elevenlabs", func(apiKey string) provider.TTSProvider {
		return NewElevenLabsTTS(apiKey)
	})
}

func (e *ElevenLabsTTS) Name() string { return "elevenlabs" }

func (e *ElevenLabsTTS) Synthesize(req *provider.TTSRequest) (*provider.TTSResult, error) {
	// ElevenLabs voice ID — the req.Voice field contains the voice ID
	voiceID := req.Voice

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

	body := map[string]any{
		"text":     text,
		"model_id": "eleven_multilingual_v2",
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
			Name:          v.VoiceID,
			Model:         "ElevenLabs",
			LanguageCodes: []string{"multilingual"},
			Gender:        gender,
			Provider:      "elevenlabs",
		}

		voices = append(voices, voice)
	}

	return voices, nil
}
