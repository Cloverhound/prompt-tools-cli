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

const openaiTTSEndpoint = "https://api.openai.com/v1/audio/speech"

const openaiNativeSampleRate = 24000 // OpenAI PCM output is 24kHz 16-bit mono

type OpenAITTS struct {
	apiKey string
}

func NewOpenAITTS(auth provider.AuthConfig) provider.TTSProvider {
	return &OpenAITTS{apiKey: auth.APIKey}
}

func init() {
	provider.RegisterTTS("openai", func(auth provider.AuthConfig) provider.TTSProvider {
		return NewOpenAITTS(auth)
	})
}

func (o *OpenAITTS) Name() string { return "openai" }

func (o *OpenAITTS) Synthesize(req *provider.TTSRequest) (*provider.TTSResult, error) {
	text := req.Text
	if req.SSML != "" {
		// OpenAI doesn't support SSML, use as plain text
		text = req.SSML
	}

	model := "gpt-4o-mini-tts"
	if req.Model != "" {
		model = req.Model
	}

	voice := req.Voice
	if voice == "" {
		voice = "alloy"
	}

	// Request PCM for WAV conversion, or mp3 for MP3 output
	responseFormat := "pcm"
	if req.Encoding == audio.EncodingMP3 {
		responseFormat = "mp3"
	}

	body := map[string]any{
		"model":           model,
		"input":           text,
		"voice":           voice,
		"response_format": responseFormat,
	}

	if req.SpeakingRate != 0 {
		body["speed"] = req.SpeakingRate
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s\n", openaiTTSEndpoint)
		fmt.Printf("[DEBUG] Body: %s\n", string(bodyJSON))
	}

	httpReq, err := http.NewRequest("POST", openaiTTSEndpoint, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI TTS API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// If MP3 was requested, return as-is
	if req.Encoding == audio.EncodingMP3 {
		return &provider.TTSResult{
			AudioData:  respBody,
			Format:     audio.EncodingMP3,
			SampleRate: 44100,
		}, nil
	}

	// PCM data from OpenAI is 24kHz 16-bit mono
	pcmData := respBody

	// Resample if target rate differs
	if req.SampleRate != openaiNativeSampleRate {
		pcmData = audio.ResampleLinear16(pcmData, openaiNativeSampleRate, req.SampleRate)
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

func (o *OpenAITTS) ListVoices(languageCode string) ([]provider.Voice, error) {
	voiceNames := []string{
		"alloy", "ash", "ballad", "coral", "echo",
		"fable", "nova", "onyx", "sage", "shimmer", "verse",
	}

	voices := make([]provider.Voice, len(voiceNames))
	for i, name := range voiceNames {
		voices[i] = provider.Voice{
			Name:              name,
			VoiceID:           name,
			Model:             "gpt-4o-mini-tts",
			LanguageCodes:     []string{"multilingual"},
			Provider:          "openai",
			NaturalSampleRate: openaiNativeSampleRate,
		}
	}

	return voices, nil
}
