package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
	"github.com/Cloverhound/prompt-tools-cli/internal/keyring"
	"github.com/Cloverhound/prompt-tools-cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show full config and provider status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}

		// Build display map with provider key status
		type configDisplay struct {
			DefaultProvider    string                           `json:"default_provider"`
			DefaultVoice       string                           `json:"default_voice"`
			DefaultSampleRate  int                              `json:"default_sample_rate"`
			DefaultEncoding    string                           `json:"default_encoding"`
			DefaultFormat      string                           `json:"default_format"`
			DefaultSTTProvider string                           `json:"default_stt_provider"`
			Providers          map[string]appconfig.ProviderConfig `json:"providers,omitempty"`
			APIKeys            map[string]string                `json:"api_keys"`
		}

		display := configDisplay{
			DefaultProvider:    cfg.DefaultProvider,
			DefaultVoice:       cfg.DefaultVoice,
			DefaultSampleRate:  cfg.DefaultSampleRate,
			DefaultEncoding:    cfg.DefaultEncoding,
			DefaultFormat:      cfg.DefaultFormat,
			DefaultSTTProvider: cfg.DefaultSTTProvider,
			Providers:          cfg.Providers,
			APIKeys:            make(map[string]string),
		}

		for _, p := range []string{"google", "elevenlabs", "assemblyai"} {
			if _, err := keyring.GetAPIKey(p); err == nil {
				display.APIKeys[p] = "configured"
			} else {
				display.APIKeys[p] = "not set"
			}
		}

		return output.PrintObject(display)
	},
}

var configSetProviderCmd = &cobra.Command{
	Use:   "set-provider <google|elevenlabs>",
	Short: "Set default TTS provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToLower(args[0])
		if provider != "google" && provider != "elevenlabs" {
			return fmt.Errorf("provider must be google or elevenlabs")
		}
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}
		cfg.DefaultProvider = provider
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Default TTS provider set to %s\n", provider)
		return nil
	},
}

var configSetSTTProviderCmd = &cobra.Command{
	Use:   "set-stt-provider <google|assemblyai>",
	Short: "Set default STT provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToLower(args[0])
		if provider != "google" && provider != "assemblyai" {
			return fmt.Errorf("provider must be google or assemblyai")
		}
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}
		cfg.DefaultSTTProvider = provider
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Default STT provider set to %s\n", provider)
		return nil
	},
}

var configSetVoiceCmd = &cobra.Command{
	Use:   "set-voice <voice-name>",
	Short: "Set default voice",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}
		cfg.DefaultVoice = args[0]
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Default voice set to %s\n", args[0])
		return nil
	},
}

var configSetFormatCmd = &cobra.Command{
	Use:   "set-format <wav|mp3>",
	Short: "Set default audio format",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := strings.ToLower(args[0])
		if format != "wav" && format != "mp3" {
			return fmt.Errorf("format must be wav or mp3")
		}
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}
		cfg.DefaultFormat = format
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Default format set to %s\n", format)
		return nil
	},
}

var configSetSampleRateCmd = &cobra.Command{
	Use:   "set-sample-rate <8000|16000|22050|24000>",
	Short: "Set default sample rate",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rateStr := args[0]
		validRates := map[string]int{"8000": 8000, "16000": 16000, "22050": 22050, "24000": 24000}
		rate, ok := validRates[rateStr]
		if !ok {
			return fmt.Errorf("sample rate must be 8000, 16000, 22050, or 24000")
		}
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}
		cfg.DefaultSampleRate = rate
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Default sample rate set to %d\n", rate)
		return nil
	},
}

var configSetEncodingCmd = &cobra.Command{
	Use:   "set-encoding <mulaw|alaw|linear16>",
	Short: "Set default encoding",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		encoding := strings.ToLower(args[0])
		valid := map[string]bool{"mulaw": true, "alaw": true, "linear16": true}
		if !valid[encoding] {
			return fmt.Errorf("encoding must be mulaw, alaw, or linear16")
		}
		cfg, err := appconfig.Load()
		if err != nil {
			return err
		}
		cfg.DefaultEncoding = encoding
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("Default encoding set to %s\n", encoding)
		return nil
	},
}

var configSetAPIKeyCmd = &cobra.Command{
	Use:   "set-api-key <google|elevenlabs|assemblyai>",
	Short: "Store API key in OS keyring (interactive)",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return []string{"google", "elevenlabs", "assemblyai"}, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToLower(args[0])
		valid := map[string]bool{"google": true, "elevenlabs": true, "assemblyai": true}
		if !valid[provider] {
			return fmt.Errorf("provider must be google, elevenlabs, or assemblyai")
		}

		fmt.Printf("Enter API key for %s: ", provider)
		keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading key: %w", err)
		}

		key := strings.TrimSpace(string(keyBytes))
		if key == "" {
			return fmt.Errorf("API key cannot be empty")
		}

		if err := keyring.SetAPIKey(provider, key); err != nil {
			return fmt.Errorf("saving key: %w", err)
		}

		fmt.Printf("API key for %s saved to system keyring\n", provider)
		return nil
	},
}

var configClearAPIKeyCmd = &cobra.Command{
	Use:   "clear-api-key <google|elevenlabs|assemblyai>",
	Short: "Remove API key from keyring",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return []string{"google", "elevenlabs", "assemblyai"}, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToLower(args[0])
		valid := map[string]bool{"google": true, "elevenlabs": true, "assemblyai": true}
		if !valid[provider] {
			return fmt.Errorf("provider must be google, elevenlabs, or assemblyai")
		}

		if err := keyring.DeleteAPIKey(provider); err != nil {
			return fmt.Errorf("removing key: %w", err)
		}

		fmt.Printf("API key for %s removed from keyring\n", provider)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetProviderCmd)
	configCmd.AddCommand(configSetSTTProviderCmd)
	configCmd.AddCommand(configSetVoiceCmd)
	configCmd.AddCommand(configSetFormatCmd)
	configCmd.AddCommand(configSetSampleRateCmd)
	configCmd.AddCommand(configSetEncodingCmd)
	configCmd.AddCommand(configSetAPIKeyCmd)
	configCmd.AddCommand(configClearAPIKeyCmd)
	rootCmd.AddCommand(configCmd)
}
