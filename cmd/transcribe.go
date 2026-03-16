package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
	_ "github.com/Cloverhound/prompt-tools-cli/internal/stt"
	"github.com/spf13/cobra"
)

var transcribeCmd = &cobra.Command{
	Use:   "transcribe",
	Short: "Transcribe an audio file to text",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile, _ := cmd.Flags().GetString("file")
		providerName, _ := cmd.Flags().GetString("provider")
		language, _ := cmd.Flags().GetString("language")
		outputFile, _ := cmd.Flags().GetString("output-file")
		timestamps, _ := cmd.Flags().GetBool("timestamps")
		phrases, _ := cmd.Flags().GetString("phrases")

		if inputFile == "" {
			return fmt.Errorf("--file is required")
		}

		// Load config for defaults
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
			fmt.Printf("Would transcribe:\n")
			fmt.Printf("  File: %s\n", inputFile)
			fmt.Printf("  Provider: %s\n", providerName)
			fmt.Printf("  Language: %s\n", language)
			if outputFile != "" {
				fmt.Printf("  Output: %s\n", outputFile)
			}
			return nil
		}

		// Resolve API key
		apiKey, err := resolveAPIKey(providerName)
		if err != nil {
			return err
		}

		sttProvider, err := provider.NewSTT(providerName, apiKey)
		if err != nil {
			return err
		}

		// Build request
		req := &provider.STTRequest{
			AudioFile:    inputFile,
			LanguageCode: language,
			Timestamps:   timestamps,
		}
		if phrases != "" {
			req.Phrases = strings.Split(phrases, ",")
		}

		result, err := sttProvider.Transcribe(req)
		if err != nil {
			return err
		}

		// Output
		if timestamps || outputFile != "" {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			if outputFile != "" {
				if err := os.WriteFile(outputFile, data, 0644); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Transcript written to %s\n", outputFile)
			} else {
				fmt.Println(string(data))
			}
		} else {
			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(result.Text+"\n"), 0644); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Transcript written to %s\n", outputFile)
			} else {
				fmt.Println(result.Text)
			}
		}

		return nil
	},
}

func init() {
	transcribeCmd.Flags().String("file", "", "Input audio file (required)")
	transcribeCmd.Flags().String("provider", "", "STT provider (google, assemblyai)")
	transcribeCmd.Flags().String("language", "en-US", "Language hint")
	transcribeCmd.Flags().String("output-file", "", "Output file (default: stdout)")
	transcribeCmd.Flags().Bool("timestamps", false, "Include word-level timestamps")
	transcribeCmd.Flags().String("phrases", "", "Comma-separated boost phrases")

	rootCmd.AddCommand(transcribeCmd)
}
