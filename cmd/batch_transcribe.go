package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
	_ "github.com/Cloverhound/prompt-tools-cli/internal/stt"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

type batchResult struct {
	File   string
	Text   string
	Result *provider.TranscriptionResult
	Err    error
}

var batchTranscribeCmd = &cobra.Command{
	Use:   "batch-transcribe",
	Short: "Batch transcribe audio files",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		filesFlag, _ := cmd.Flags().GetString("files")
		globPattern, _ := cmd.Flags().GetString("glob")
		providerName, _ := cmd.Flags().GetString("provider")
		language, _ := cmd.Flags().GetString("language")
		outputDir, _ := cmd.Flags().GetString("output-dir")
		outputFormat, _ := cmd.Flags().GetString("output-format")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		continueOnError, _ := cmd.Flags().GetBool("continue-on-error")

		// Collect input files
		var inputFiles []string
		if filesFlag != "" {
			inputFiles = strings.Split(filesFlag, ",")
		} else if dir != "" {
			pattern := "*.wav"
			if globPattern != "" {
				pattern = globPattern
			}
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				return fmt.Errorf("glob error: %w", err)
			}
			inputFiles = matches
		} else {
			return fmt.Errorf("specify --dir or --files")
		}

		if len(inputFiles) == 0 {
			fmt.Fprintln(os.Stderr, "No input files found")
			return nil
		}

		// Load config
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}

		if providerName == "" {
			providerName = cfg.DefaultSTTProvider
			if providerName == "" {
				providerName = "google"
			}
		}

		// Dry-run
		if config.DryRun() {
			fmt.Printf("Would transcribe %d files:\n", len(inputFiles))
			for _, f := range inputFiles {
				fmt.Printf("  %s\n", f)
			}
			return nil
		}

		// Resolve auth
		auth, err := resolveAuth(providerName)
		if err != nil {
			return err
		}

		sttProvider, err := provider.NewSTT(providerName, auth)
		if err != nil {
			return err
		}

		if outputDir != "" {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return err
			}
		}

		if concurrency < 1 {
			concurrency = 5
		}

		results := make([]batchResult, len(inputFiles))
		var succeeded, failed int64

		bar := progressbar.NewOptions(len(inputFiles),
			progressbar.OptionSetDescription("Transcribing"),
			progressbar.OptionShowCount(),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionClearOnFinish(),
		)

		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup

		for i, file := range inputFiles {
			i, file := i, file
			wg.Add(1)
			sem <- struct{}{}

			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				defer bar.Add(1)

				req := &provider.STTRequest{
					AudioFile:    file,
					LanguageCode: language,
				}

				txResult, err := sttProvider.Transcribe(req)
				results[i] = batchResult{File: file, Result: txResult, Err: err}
				if err != nil {
					atomic.AddInt64(&failed, 1)
				} else {
					atomic.AddInt64(&succeeded, 1)
					results[i].Text = txResult.Text
				}
			}()
		}

		wg.Wait()
		bar.Finish()

		// Write results
		switch outputFormat {
		case "json":
			if err := writeJSONResults(results, outputDir); err != nil {
				return err
			}
		case "csv":
			if err := writeCSVResults(results, outputDir); err != nil {
				return err
			}
		default: // text
			if err := writeTextResults(results, outputDir); err != nil {
				return err
			}
		}

		// Summary
		fmt.Fprintf(os.Stderr, "\nComplete: %d succeeded, %d failed (of %d total)\n",
			succeeded, failed, len(inputFiles))

		if failed > 0 {
			fmt.Fprintln(os.Stderr, "\nErrors:")
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(os.Stderr, "  %s: %s\n", r.File, r.Err)
				}
			}
			if !continueOnError {
				return fmt.Errorf("%d transcriptions failed", failed)
			}
		}

		return nil
	},
}

func writeTextResults(results []batchResult, outputDir string) error {
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		if outputDir != "" {
			base := strings.TrimSuffix(filepath.Base(r.File), filepath.Ext(r.File))
			outPath := filepath.Join(outputDir, base+".txt")
			if err := os.WriteFile(outPath, []byte(r.Text+"\n"), 0644); err != nil {
				return err
			}
		} else {
			fmt.Printf("%s: %s\n", filepath.Base(r.File), r.Text)
		}
	}
	return nil
}

func writeJSONResults(results []batchResult, outputDir string) error {
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		data, err := json.MarshalIndent(r.Result, "", "  ")
		if err != nil {
			return err
		}
		if outputDir != "" {
			base := strings.TrimSuffix(filepath.Base(r.File), filepath.Ext(r.File))
			outPath := filepath.Join(outputDir, base+".json")
			if err := os.WriteFile(outPath, data, 0644); err != nil {
				return err
			}
		} else {
			fmt.Printf("--- %s ---\n%s\n", filepath.Base(r.File), string(data))
		}
	}
	return nil
}

func writeCSVResults(results []batchResult, outputDir string) error {
	outPath := "transcriptions.csv"
	if outputDir != "" {
		outPath = filepath.Join(outputDir, "transcriptions.csv")
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Write([]string{"File", "Transcript", "Confidence"})
	for _, r := range results {
		if r.Err != nil {
			w.Write([]string{filepath.Base(r.File), "ERROR: " + r.Err.Error(), ""})
			continue
		}
		w.Write([]string{filepath.Base(r.File), r.Text, fmt.Sprintf("%.4f", r.Result.Confidence)})
	}
	w.Flush()
	return w.Error()
}

func init() {
	batchTranscribeCmd.Flags().String("dir", "", "Directory of audio files")
	batchTranscribeCmd.Flags().String("files", "", "Comma-separated file list")
	batchTranscribeCmd.Flags().String("glob", "*.wav", "File pattern")
	batchTranscribeCmd.Flags().String("provider", "", "STT provider (google, assemblyai, openai)")
	batchTranscribeCmd.Flags().String("language", "en-US", "Language hint")
	batchTranscribeCmd.Flags().String("output-dir", "", "Output directory")
	batchTranscribeCmd.Flags().String("output-format", "text", "Output format: text, json, csv")
	batchTranscribeCmd.Flags().Int("concurrency", 5, "Parallel transcriptions")
	batchTranscribeCmd.Flags().Bool("continue-on-error", false, "Keep going on failure")

	rootCmd.AddCommand(batchTranscribeCmd)
}
