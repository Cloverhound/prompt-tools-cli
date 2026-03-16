package stt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
)

const googleSTTEndpoint = "https://speech.googleapis.com/v1"

type GoogleSTT struct {
	apiKey string
}

func NewGoogleSTT(apiKey string) provider.STTProvider {
	return &GoogleSTT{apiKey: apiKey}
}

func init() {
	provider.RegisterSTT("google", func(apiKey string) provider.STTProvider {
		return NewGoogleSTT(apiKey)
	})
}

func (g *GoogleSTT) Name() string { return "google" }

func (g *GoogleSTT) Transcribe(req *provider.STTRequest) (*provider.TranscriptionResult, error) {
	// Read audio data
	audioData := req.AudioData
	if len(audioData) == 0 && req.AudioFile != "" {
		var err error
		audioData, err = os.ReadFile(req.AudioFile)
		if err != nil {
			return nil, fmt.Errorf("reading audio file: %w", err)
		}
	}

	langCode := req.LanguageCode
	if langCode == "" {
		langCode = "en-US"
	}

	// Build recognition config
	recognitionConfig := map[string]any{
		"languageCode":         langCode,
		"enableWordTimeOffsets": req.Timestamps,
	}

	if len(req.Phrases) > 0 {
		recognitionConfig["speechContexts"] = []map[string]any{
			{"phrases": req.Phrases},
		}
	}

	body := map[string]any{
		"config": recognitionConfig,
		"audio": map[string]string{
			"content": base64.StdEncoding.EncodeToString(audioData),
		},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/speech:recognize?key=%s", googleSTTEndpoint, g.apiKey)

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s\n", url)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, fmt.Errorf("Google STT request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google STT API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Results []struct {
			Alternatives []struct {
				Transcript string  `json:"transcript"`
				Confidence float64 `json:"confidence"`
				Words      []struct {
					Word      string `json:"word"`
					StartTime string `json:"startTime"`
					EndTime   string `json:"endTime"`
				} `json:"words"`
			} `json:"alternatives"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	txResult := &provider.TranscriptionResult{}
	var allText []string
	for _, r := range result.Results {
		if len(r.Alternatives) > 0 {
			alt := r.Alternatives[0]
			allText = append(allText, alt.Transcript)
			if alt.Confidence > txResult.Confidence {
				txResult.Confidence = alt.Confidence
			}
			for _, w := range alt.Words {
				txResult.Words = append(txResult.Words, provider.WordTimestamp{
					Word:      w.Word,
					StartTime: parseDuration(w.StartTime),
					EndTime:   parseDuration(w.EndTime),
				})
			}
		}
	}
	txResult.Text = strings.Join(allText, " ")

	return txResult, nil
}

// parseDuration parses Google's duration format "1.500s" to float64 seconds.
func parseDuration(s string) float64 {
	s = strings.TrimSuffix(s, "s")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
