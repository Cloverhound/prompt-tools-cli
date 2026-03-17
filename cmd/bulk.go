package cmd

import (
	"fmt"
	"os"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
	"github.com/Cloverhound/prompt-tools-cli/internal/bulk"
	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
	_ "github.com/Cloverhound/prompt-tools-cli/internal/tts"
	"github.com/spf13/cobra"
)

var bulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Bulk prompt generation",
}

var bulkGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Process spreadsheet/CSV to audio files",
	Long: `Read a spreadsheet (.xlsx or .csv) and generate one audio file per row.

Input format (row 1 = headers, skipped automatically):

  Filename        | Voice             | Text                  | SSML | Sample Rate | Encoding | Notes
  welcome.wav     | en-US-Chirp3-HD-Achernar   | Welcome to support.   | no   |             |          | Main greeting
  #holiday.wav    | en-US-Chirp3-HD-Achernar   | Closed for holiday.   | no   |             |          | Skipped
  es/welcome.wav  | es-MX-Chirp3-HD-A | Bienvenido.           | no   |             |          | Subdirectory

  - Rows starting with # are skipped.
  - Voice, Sample Rate, and Encoding are optional (defaults from config).
  - SSML column: yes/no — whether Text contains SSML markup.
  - Filename supports subdirectories (e.g., en-US/welcome.wav) — folders are created automatically.

Examples:
  prompt-tools bulk generate --file prompts.xlsx --output-dir ./output
  prompt-tools bulk generate --file prompts.csv --output-dir ./output --concurrency 10
  prompt-tools bulk generate --file prompts.xlsx --output-dir ./output --skip-existing
  prompt-tools bulk generate --file prompts.xlsx --output-dir ./output --continue-on-error`,
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile, _ := cmd.Flags().GetString("file")
		sheet, _ := cmd.Flags().GetString("sheet")
		outputDir, _ := cmd.Flags().GetString("output-dir")
		providerName, _ := cmd.Flags().GetString("provider")
		format, _ := cmd.Flags().GetString("format")
		sampleRate, _ := cmd.Flags().GetInt("sample-rate")
		encoding, _ := cmd.Flags().GetString("encoding")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		skipExisting, _ := cmd.Flags().GetBool("skip-existing")
		continueOnError, _ := cmd.Flags().GetBool("continue-on-error")

		if inputFile == "" {
			return fmt.Errorf("--file is required")
		}

		// Load config for defaults
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}

		if providerName == "" {
			providerName = cfg.DefaultProvider
			if providerName == "" {
				providerName = "google"
			}
		}
		if format == "" {
			format = cfg.DefaultFormat
			if format == "" {
				format = "wav"
			}
		}
		if sampleRate == 0 {
			sampleRate = cfg.DefaultSampleRate
			if sampleRate == 0 {
				sampleRate = 8000
			}
		}
		if encoding == "" {
			encoding = cfg.DefaultEncoding
			if encoding == "" {
				encoding = "mulaw"
			}
		}
		if outputDir == "" {
			outputDir = "./output"
		}

		// Parse spreadsheet
		rows, err := bulk.ParseFile(inputFile, sheet)
		if err != nil {
			return err
		}

		if len(rows) == 0 {
			fmt.Fprintln(os.Stderr, "No rows to process")
			return nil
		}

		// Validate
		if errs := bulk.ValidateRows(rows); len(errs) > 0 {
			fmt.Fprintln(os.Stderr, "Validation errors:")
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  %s\n", e)
			}
			return fmt.Errorf("fix validation errors and retry")
		}

		// Dry-run
		if config.DryRun() {
			fmt.Printf("Would generate %d prompts:\n", len(rows))
			fmt.Printf("  Provider: %s\n", providerName)
			fmt.Printf("  Format: %s\n", format)
			fmt.Printf("  Sample rate: %d Hz\n", sampleRate)
			fmt.Printf("  Encoding: %s\n", encoding)
			fmt.Printf("  Output dir: %s\n", outputDir)
			fmt.Printf("  Concurrency: %d\n", concurrency)
			for _, row := range rows {
				fmt.Printf("  - %s: %s\n", row.Filename, truncate(row.Text, 60))
			}
			return nil
		}

		// Resolve API key and create provider
		apiKey, err := resolveAPIKey(providerName)
		if err != nil {
			return err
		}

		tts, err := provider.NewTTS(providerName, apiKey)
		if err != nil {
			return err
		}

		defaultVoice := cfg.DefaultVoice
		if defaultVoice == "" || (cfg.DefaultProvider != providerName && cfg.DefaultProvider != "") {
			switch providerName {
			case "openai":
				defaultVoice = "alloy"
			case "elevenlabs":
				defaultVoice = "Sarah"
			default:
				defaultVoice = "en-US-Chirp3-HD-Achernar"
			}
		}

		// Run pipeline
		result, err := bulk.Run(rows, tts, bulk.PipelineConfig{
			OutputDir:       outputDir,
			Concurrency:     concurrency,
			SkipExisting:    skipExisting,
			ContinueOnError: continueOnError,
			DefaultVoice:    defaultVoice,
			DefaultRate:     sampleRate,
			DefaultEncoding: encoding,
			DefaultFormat:   format,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "\nComplete: %d succeeded, %d skipped, %d failed (of %d total)\n",
			result.Succeeded, result.Skipped, result.Failed, result.Total)

		if len(result.Errors) > 0 {
			fmt.Fprintln(os.Stderr, "\nErrors:")
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  %s\n", e)
			}
		}

		if result.Failed > 0 && !continueOnError {
			return fmt.Errorf("%d prompts failed", result.Failed)
		}

		return nil
	},
}

var bulkTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "Generate blank template",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputPath, _ := cmd.Flags().GetString("output")
		if outputPath == "" {
			outputPath = "template.xlsx"
		}

		if err := bulk.GenerateTemplate(outputPath); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Template written to %s\n", outputPath)
		return nil
	},
}

var bulkValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate input without generating",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile, _ := cmd.Flags().GetString("file")
		if inputFile == "" {
			return fmt.Errorf("--file is required")
		}

		sheet, _ := cmd.Flags().GetString("sheet")
		rows, err := bulk.ParseFile(inputFile, sheet)
		if err != nil {
			return err
		}

		if len(rows) == 0 {
			fmt.Fprintln(os.Stderr, "No rows found")
			return nil
		}

		errs := bulk.ValidateRows(rows)
		if len(errs) > 0 {
			fmt.Fprintln(os.Stderr, "Validation errors:")
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  %s\n", e)
			}
			return fmt.Errorf("%d validation errors", len(errs))
		}

		fmt.Fprintf(os.Stderr, "Valid: %d rows ready for generation\n", len(rows))
		return nil
	},
}

func init() {
	bulkGenerateCmd.Flags().String("file", "", "Input spreadsheet (.xlsx or .csv)")
	bulkGenerateCmd.Flags().String("sheet", "", "Sheet name (xlsx only)")
	bulkGenerateCmd.Flags().String("output-dir", "./output", "Output directory")
	bulkGenerateCmd.Flags().String("provider", "", "TTS provider override")
	bulkGenerateCmd.Flags().String("format", "", "Audio format override")
	bulkGenerateCmd.Flags().Int("sample-rate", 0, "Sample rate override")
	bulkGenerateCmd.Flags().String("encoding", "", "Encoding override")
	bulkGenerateCmd.Flags().Int("concurrency", 5, "Parallel API requests")
	bulkGenerateCmd.Flags().Bool("skip-existing", false, "Don't regenerate existing files")
	bulkGenerateCmd.Flags().Bool("continue-on-error", false, "Keep going on row failure")

	bulkTemplateCmd.Flags().StringP("output", "o", "template.xlsx", "Output file (.xlsx or .csv)")

	bulkValidateCmd.Flags().String("file", "", "Input spreadsheet to validate")
	bulkValidateCmd.Flags().String("sheet", "", "Sheet name (xlsx only)")

	bulkCmd.AddCommand(bulkGenerateCmd)
	bulkCmd.AddCommand(bulkTemplateCmd)
	bulkCmd.AddCommand(bulkValidateCmd)
	rootCmd.AddCommand(bulkCmd)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
