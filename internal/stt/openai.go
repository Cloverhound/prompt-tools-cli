package stt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
)

const openaiSTTEndpoint = "https://api.openai.com/v1/audio/transcriptions"

type OpenAISTT struct {
	apiKey string
}

func NewOpenAISTT(apiKey string) provider.STTProvider {
	return &OpenAISTT{apiKey: apiKey}
}

func init() {
	provider.RegisterSTT("openai", func(apiKey string) provider.STTProvider {
		return NewOpenAISTT(apiKey)
	})
}

func (o *OpenAISTT) Name() string { return "openai" }

func (o *OpenAISTT) Transcribe(req *provider.STTRequest) (*provider.TranscriptionResult, error) {
	// Read audio data
	audioData := req.AudioData
	if len(audioData) == 0 && req.AudioFile != "" {
		var err error
		audioData, err = os.ReadFile(req.AudioFile)
		if err != nil {
			return nil, fmt.Errorf("reading audio file: %w", err)
		}
	}

	// Determine filename for the multipart upload
	filename := "audio.wav"
	if req.AudioFile != "" {
		filename = filepath.Base(req.AudioFile)
	}

	// Build multipart form
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// File field
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("writing audio data: %w", err)
	}

	// Model field — gpt-4o-mini-transcribe doesn't support verbose_json/timestamps,
	// so fall back to whisper-1 when timestamps are requested
	model := "gpt-4o-mini-transcribe"
	if req.Timestamps {
		model = "whisper-1"
	}
	if err := w.WriteField("model", model); err != nil {
		return nil, err
	}

	// Response format
	responseFormat := "json"
	if req.Timestamps {
		responseFormat = "verbose_json"
	}
	if err := w.WriteField("response_format", responseFormat); err != nil {
		return nil, err
	}

	// Timestamp granularities (for verbose_json with whisper-1)
	if req.Timestamps {
		if err := w.WriteField("timestamp_granularities[]", "word"); err != nil {
			return nil, err
		}
	}

	// Language hint
	if req.LanguageCode != "" {
		// OpenAI expects ISO-639-1 language codes (e.g., "en" not "en-US")
		lang := req.LanguageCode
		if idx := strings.Index(lang, "-"); idx > 0 {
			lang = lang[:idx]
		}
		if err := w.WriteField("language", lang); err != nil {
			return nil, err
		}
	}

	// Boost phrases as prompt
	if len(req.Phrases) > 0 {
		if err := w.WriteField("prompt", strings.Join(req.Phrases, " ")); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s (%d bytes audio, model=%s)\n", openaiSTTEndpoint, len(audioData), model)
	}

	httpReq, err := http.NewRequest("POST", openaiSTTEndpoint, &buf)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI STT request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI STT API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result struct {
		Text  string `json:"text"`
		Words []struct {
			Word  string  `json:"word"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
		} `json:"words"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	txResult := &provider.TranscriptionResult{
		Text: result.Text,
	}

	if req.Timestamps {
		for _, w := range result.Words {
			txResult.Words = append(txResult.Words, provider.WordTimestamp{
				Word:      w.Word,
				StartTime: w.Start,
				EndTime:   w.End,
			})
		}
	}

	return txResult, nil
}
