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

const (
	googleTTSEndpoint      = "https://texttospeech.googleapis.com/v1"
	geminiAPIEndpoint      = "https://generativelanguage.googleapis.com/v1beta"
	geminiNativeSampleRate = 24000 // Gemini API always returns 24kHz linear16 PCM
)

type GoogleTTS struct {
	apiKey      string
	bearerToken string
	project     string
}

func NewGoogleTTS(auth provider.AuthConfig) provider.TTSProvider {
	return &GoogleTTS{apiKey: auth.APIKey, bearerToken: auth.BearerToken, project: auth.Project}
}

// setProjectHeader adds the x-goog-user-project header if a project is configured.
func (g *GoogleTTS) setProjectHeader(req *http.Request) {
	if g.project != "" {
		req.Header.Set("x-goog-user-project", g.project)
	}
}

func init() {
	provider.RegisterTTS("google", func(auth provider.AuthConfig) provider.TTSProvider {
		return NewGoogleTTS(auth)
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
	if isGeminiVoice(req.Voice) || req.Model != "" {
		// Gemini voice with OAuth2 → Cloud TTS endpoint (enables --style, server-side encoding)
		if g.bearerToken != "" {
			return g.synthesizeCloudTTSGemini(req)
		}
		// Gemini voice with API key → Generative Language API
		return g.synthesizeGemini(req)
	}
	return g.synthesizeCloudTTS(req)
}

// synthesizeCloudTTSGemini uses the Cloud TTS endpoint with Bearer auth for Gemini voices.
// This enables the input.prompt field for voice steering and server-side audio encoding.
func (g *GoogleTTS) synthesizeCloudTTSGemini(req *provider.TTSRequest) (*provider.TTSResult, error) {
	input := map[string]string{}
	if req.SSML != "" {
		input["ssml"] = req.SSML
	} else {
		input["text"] = req.Text
	}
	if req.Style != "" {
		input["prompt"] = req.Style
	}

	// Resolve model — use explicit model, or default to gemini-2.5-flash-tts (no -preview suffix for Cloud TTS)
	model := req.Model
	if model == "" {
		model = "gemini-2.5-flash-tts"
	}
	// Strip -preview suffix if present — Cloud TTS endpoint uses non-preview model names
	model = strings.Replace(model, "-preview-tts", "-tts", 1)

	langCode := req.LanguageCode
	if langCode == "" {
		langCode = "en-US"
	}

	voice := map[string]string{
		"languageCode": langCode,
		"name":         req.Voice,
		"modelName":    model,
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

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s/text:synthesize (Cloud TTS Gemini, Bearer auth)\n", googleTTSEndpoint)
		fmt.Printf("[DEBUG] Body: %s\n", string(bodyJSON))
	}

	url := fmt.Sprintf("%s/text:synthesize", googleTTSEndpoint)
	httpReq, err := http.NewRequest("POST", url, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.bearerToken)
	g.setProjectHeader(httpReq)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Cloud TTS Gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Cloud TTS Gemini API error (%d): %s", resp.StatusCode, string(respBody))
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

	// Server-side encoding — audio is already in the requested format
	return &provider.TTSResult{
		AudioData:  audioData,
		Format:     req.Encoding,
		SampleRate: req.SampleRate,
	}, nil
}

// synthesizeGemini uses the Generative Language API for Gemini TTS voices.
// Returns audio resampled and encoded to match the request parameters.
// resolveGeminiModel returns the model to use for Gemini TTS.
// If explicit, uses that. Otherwise queries the API for available TTS models
// and picks the best one (prefers "pro" over "flash").
func (g *GoogleTTS) resolveGeminiModel() (string, error) {
	url := fmt.Sprintf("%s/models?key=%s", geminiAPIEndpoint, g.apiKey)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("listing Gemini models: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading model list: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini models API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Models []struct {
			Name                       string   `json:"name"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing model list: %w", err)
	}

	// Filter to TTS models that support generateContent
	var ttsModels []string
	for _, m := range result.Models {
		if !strings.HasSuffix(m.Name, "-tts") {
			continue
		}
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				// Strip "models/" prefix
				name := strings.TrimPrefix(m.Name, "models/")
				ttsModels = append(ttsModels, name)
				break
			}
		}
	}

	if len(ttsModels) == 0 {
		return "", fmt.Errorf("no Gemini TTS models available")
	}

	// Prefer "pro" over others
	for _, m := range ttsModels {
		if strings.Contains(m, "pro") {
			return m, nil
		}
	}
	return ttsModels[0], nil
}

func (g *GoogleTTS) synthesizeGemini(req *provider.TTSRequest) (*provider.TTSResult, error) {
	model := req.Model
	if model == "" {
		var err error
		model, err = g.resolveGeminiModel()
		if err != nil {
			return nil, err
		}
		if config.Debug() {
			fmt.Printf("[DEBUG] Auto-selected Gemini TTS model: %s\n", model)
		}
	}

	// Build the text content — Gemini TTS doesn't support SSML, so strip tags
	text := req.Text
	if req.SSML != "" {
		text = req.SSML
	}

	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": text},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]any{
				"voiceConfig": map[string]any{
					"prebuiltVoiceConfig": map[string]string{
						"voiceName": req.Voice,
					},
				},
			},
		},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", geminiAPIEndpoint, model, g.apiKey)

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s/models/%s:generateContent\n", geminiAPIEndpoint, model)
		fmt.Printf("[DEBUG] Body: %s\n", string(bodyJSON))
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, fmt.Errorf("Gemini TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini TTS API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse Gemini response: candidates[0].content.parts[0].inlineData.data
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing Gemini response: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("Gemini TTS returned no audio data")
	}

	pcmData, err := base64.StdEncoding.DecodeString(result.Candidates[0].Content.Parts[0].InlineData.Data)
	if err != nil {
		return nil, fmt.Errorf("decoding Gemini audio: %w", err)
	}

	if config.Debug() {
		mime := result.Candidates[0].Content.Parts[0].InlineData.MimeType
		fmt.Printf("[DEBUG] Gemini response: %s, %d bytes PCM\n", mime, len(pcmData))
	}

	// Gemini returns 24kHz linear16 PCM. Resample to requested rate.
	if req.SampleRate != geminiNativeSampleRate {
		pcmData = audio.ResampleLinear16(pcmData, geminiNativeSampleRate, req.SampleRate)
	}

	// Convert encoding if needed
	outputData := pcmData
	outputFormat := audio.EncodingLinear16
	switch req.Encoding {
	case audio.EncodingMulaw:
		outputData = audio.Linear16ToMulaw(pcmData)
		outputFormat = audio.EncodingMulaw
	case audio.EncodingAlaw:
		outputData = audio.Linear16ToAlaw(pcmData)
		outputFormat = audio.EncodingAlaw
	case audio.EncodingLinear16:
		// already linear16
	}

	return &provider.TTSResult{
		AudioData:  outputData,
		Format:     outputFormat,
		SampleRate: req.SampleRate,
	}, nil
}

// synthesizeCloudTTS uses the standard Google Cloud Text-to-Speech API.
func (g *GoogleTTS) synthesizeCloudTTS(req *provider.TTSRequest) (*provider.TTSResult, error) {
	var input map[string]string
	if req.SSML != "" {
		input = map[string]string{"ssml": req.SSML}
	} else {
		input = map[string]string{"text": req.Text}
	}

	langCode := "en-US"
	parts := strings.SplitN(req.Voice, "-", 3)
	if len(parts) >= 2 {
		langCode = parts[0] + "-" + parts[1]
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

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	if config.Debug() {
		fmt.Printf("[DEBUG] POST %s/text:synthesize\n", googleTTSEndpoint)
		fmt.Printf("[DEBUG] Body: %s\n", string(bodyJSON))
	}

	var httpReq *http.Request
	if g.bearerToken != "" {
		httpReq, err = http.NewRequest("POST", fmt.Sprintf("%s/text:synthesize", googleTTSEndpoint), strings.NewReader(string(bodyJSON)))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+g.bearerToken)
	} else {
		url := fmt.Sprintf("%s/text:synthesize?key=%s", googleTTSEndpoint, g.apiKey)
		httpReq, err = http.NewRequest("POST", url, strings.NewReader(string(bodyJSON)))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
	}
	g.setProjectHeader(httpReq)

	resp, err := http.DefaultClient.Do(httpReq)
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
	var httpReq *http.Request
	var err error

	if g.bearerToken != "" {
		reqURL := fmt.Sprintf("%s/voices", googleTTSEndpoint)
		if languageCode != "" {
			reqURL += "?languageCode=" + languageCode
		}
		httpReq, err = http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+g.bearerToken)
	} else {
		reqURL := fmt.Sprintf("%s/voices?key=%s", googleTTSEndpoint, g.apiKey)
		if languageCode != "" {
			reqURL += "&languageCode=" + languageCode
		}
		httpReq, err = http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}
	}
	g.setProjectHeader(httpReq)

	if config.Debug() {
		fmt.Printf("[DEBUG] GET %s\n", httpReq.URL.String())
	}

	resp, err := http.DefaultClient.Do(httpReq)
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
