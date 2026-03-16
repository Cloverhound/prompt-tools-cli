package cmd

import (
	"fmt"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/output"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "prompt-tools",
	Short: "Prompt Tools CLI — IVR prompt generation and transcription",
	Long:  `A command-line interface for generating IVR prompts using text-to-speech and transcribing audio files.`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		debug, _ := cmd.Flags().GetBool("debug")
		config.SetDebug(debug)

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		config.SetDryRun(dryRun)

		format, _ := cmd.Flags().GetString("output")
		output.SetFormat(format)

		// Load app config (needed by commands that check defaults)
		_, err := appconfig.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Skip API key validation for commands that don't need it
		if skipAuth(cmd) {
			return nil
		}

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().String("output", "json", "Output format: json, table, csv, raw")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging of HTTP requests")
	rootCmd.PersistentFlags().Bool("dry-run", false, "Show plan without executing")

	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		cmd.PrintErrf("Error: %s\n\n", err)
		cmd.PrintErr(cmd.UsageString())
		cmd.Root().SilenceErrors = true
		return err
	})
}

func skipAuth(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "config", "version", "update", "setup", "help", "prompt-tools", "template":
			if c.Name() == "prompt-tools" {
				continue
			}
			return true
		}
	}
	name := cmd.Name()
	if name == "help" || name == "prompt-tools" {
		return true
	}
	return false
}
