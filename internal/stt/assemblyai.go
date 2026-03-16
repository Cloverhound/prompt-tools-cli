package stt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
)

const assemblyAIEndpoint = "https://api.assemblyai.com/v2"

type AssemblyAISTT struct {
	apiKey string
}

func NewAssemblyAISTT(apiKey string) provider.STTProvider {
	return &AssemblyAISTT{apiKey: apiKey}
}

func init() {
	provider.RegisterSTT("assemblyai", func(apiKey string) provider.STTProvider {
		return NewAssemblyAISTT(apiKey)
	})
}

func (a *AssemblyAISTT) Name() string { return "assemblyai" }

func (a *AssemblyAISTT) Transcribe(req *provider.STTRequest) (*provider.TranscriptionResult, error) {
	// Read audio data
	audioData := req.AudioData
	if len(audioData) == 0 && req.AudioFile != "" {
		var err error
		audioData, err = os.ReadFile(req.AudioFile)
		if err != nil {
			return nil, fmt.Errorf("reading audio file: %w", err)
		}
	}

	// Step 1: Upload audio
	uploadURL, err := a.upload(audioData)
	if err != nil {
		return nil, fmt.Errorf("uploading audio: %w", err)
	}

	// Step 2: Create transcription
	txBody := map[string]any{
		"audio_url": uploadURL,
	}
	if req.LanguageCode != "" {
		txBody["language_code"] = req.LanguageCode
	}
	if req.Timestamps {
		// AssemblyAI always includes word timestamps by default
	}
	if len(req.Phrases) > 0 {
		txBody["word_boost"] = req.Phrases
	}

	txJSON, err := json.Marshal(txBody)
	if err != nil {
		return nil, err
	}

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s/transcript\n", assemblyAIEndpoint)
	}

	httpReq, err := http.NewRequest("POST", assemblyAIEndpoint+"/transcript", strings.NewReader(string(txJSON)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", a.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("creating transcription: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AssemblyAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var txResp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &txResp); err != nil {
		return nil, err
	}

	// Step 3: Poll for completion
	return a.poll(txResp.ID, req.Timestamps)
}

func (a *AssemblyAISTT) upload(data []byte) (string, error) {
	url := assemblyAIEndpoint + "/upload"

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s (%d bytes)\n", url, len(data))
	}

	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", a.apiKey)
	httpReq.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.UploadURL, nil
}

func (a *AssemblyAISTT) poll(transcriptID string, timestamps bool) (*provider.TranscriptionResult, error) {
	url := fmt.Sprintf("%s/transcript/%s", assemblyAIEndpoint, transcriptID)

	for {
		if config.Debug() {
			fmt.Printf("[DEBUG] GET %s\n", url)
		}

		httpReq, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", a.apiKey)

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var result struct {
			Status     string  `json:"status"`
			Text       string  `json:"text"`
			Confidence float64 `json:"confidence"`
			Words      []struct {
				Text  string `json:"text"`
				Start int    `json:"start"`
				End   int    `json:"end"`
			} `json:"words"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		switch result.Status {
		case "completed":
			txResult := &provider.TranscriptionResult{
				Text:       result.Text,
				Confidence: result.Confidence,
			}
			if timestamps {
				for _, w := range result.Words {
					txResult.Words = append(txResult.Words, provider.WordTimestamp{
						Word:      w.Text,
						StartTime: float64(w.Start) / 1000.0,
						EndTime:   float64(w.End) / 1000.0,
					})
				}
			}
			return txResult, nil
		case "error":
			return nil, fmt.Errorf("transcription failed: %s", result.Error)
		default:
			time.Sleep(2 * time.Second)
		}
	}
}
