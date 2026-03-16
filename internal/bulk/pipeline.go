package bulk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Cloverhound/prompt-tools-cli/internal/audio"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
	"github.com/schollz/progressbar/v3"
)

// PipelineConfig contains settings for bulk processing.
type PipelineConfig struct {
	OutputDir       string
	Concurrency     int
	SkipExisting    bool
	ContinueOnError bool
	DefaultVoice    string
	DefaultRate     int
	DefaultEncoding string
	DefaultFormat   string
}

// PipelineResult contains the results of bulk processing.
type PipelineResult struct {
	Total     int
	Succeeded int
	Skipped   int
	Failed    int
	Errors    []error
}

// Run processes all rows through the TTS provider.
func Run(rows []Row, tts provider.TTSProvider, cfg PipelineConfig) (*PipelineResult, error) {
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 5
	}

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	result := &PipelineResult{Total: len(rows)}
	var mu sync.Mutex
	var succeeded, skipped, failed int64

	bar := progressbar.NewOptions(len(rows),
		progressbar.OptionSetDescription("Generating prompts"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionClearOnFinish(),
	)

	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup

	for _, row := range rows {
		row := row
		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer bar.Add(1)

			outPath := filepath.Join(cfg.OutputDir, row.Filename)

			// Skip existing
			if cfg.SkipExisting {
				if _, err := os.Stat(outPath); err == nil {
					atomic.AddInt64(&skipped, 1)
					return
				}
			}

			// Resolve defaults
			voice := row.Voice
			if voice == "" {
				voice = cfg.DefaultVoice
			}
			sampleRate := row.SampleRate
			if sampleRate == 0 {
				sampleRate = cfg.DefaultRate
			}
			encoding := row.Encoding
			if encoding == "" {
				encoding = cfg.DefaultEncoding
			}

			// Build TTS request
			ttsReq := &provider.TTSRequest{
				Voice:      voice,
				SampleRate: sampleRate,
				Encoding:   encoding,
			}
			if row.IsSSML {
				ttsReq.SSML = row.Text
			} else {
				ttsReq.Text = row.Text
			}

			// Synthesize
			ttsResult, err := tts.Synthesize(ttsReq)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Errorf("row %d (%s): %w", row.LineNumber, row.Filename, err))
				mu.Unlock()
				if !cfg.ContinueOnError {
					return
				}
				return
			}

			// Write output file
			outputData := ttsResult.AudioData
			ext := strings.ToLower(filepath.Ext(row.Filename))
			if ext == ".wav" && ttsResult.Format != audio.EncodingMP3 {
				outputData, err = audio.WriteWAV(ttsResult.AudioData, ttsResult.SampleRate, ttsResult.Format)
				if err != nil {
					atomic.AddInt64(&failed, 1)
					mu.Lock()
					result.Errors = append(result.Errors, fmt.Errorf("row %d (%s): writing WAV: %w", row.LineNumber, row.Filename, err))
					mu.Unlock()
					return
				}
			}

			// Ensure output subdirectories exist
			if dir := filepath.Dir(outPath); dir != cfg.OutputDir {
				os.MkdirAll(dir, 0755)
			}

			if err := os.WriteFile(outPath, outputData, 0644); err != nil {
				atomic.AddInt64(&failed, 1)
				mu.Lock()
				result.Errors = append(result.Errors, fmt.Errorf("row %d (%s): writing file: %w", row.LineNumber, row.Filename, err))
				mu.Unlock()
				return
			}

			atomic.AddInt64(&succeeded, 1)
		}()
	}

	wg.Wait()
	bar.Finish()

	result.Succeeded = int(succeeded)
	result.Skipped = int(skipped)
	result.Failed = int(failed)

	return result, nil
}
