package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
	"github.com/Cloverhound/prompt-tools-cli/internal/gcpauth"
	"github.com/Cloverhound/prompt-tools-cli/internal/keyring"
	"github.com/Cloverhound/prompt-tools-cli/internal/output"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
	_ "github.com/Cloverhound/prompt-tools-cli/internal/tts"
	"github.com/spf13/cobra"
)

var voicesCmd = &cobra.Command{
	Use:   "voices",
	Short: "List available TTS voices",
	Long: `List available TTS voices from a provider, with optional language and gender filters.

Google voice models (highest to lowest quality):

  Model         Name Pattern                    Notes
  ─────         ────────────                    ─────
  Gemini        Achernar, Kore, Puck            Bare names, highest quality
  Chirp3-HD     en-US-Chirp3-HD-Achernar        Same voices as Gemini, different model
  Chirp-HD      en-US-Chirp-HD-D                HD voices
  Studio        en-US-Studio-O                  Studio-grade voices
  Neural2       en-US-Neural2-F                 Neural voices
  Wavenet       en-US-Wavenet-A                 DeepMind Wavenet
  Standard      en-US-Standard-A                Basic concatenative

Gemini voices use the same star/moon names as Chirp3-HD but are a distinct model
requiring a model_name parameter (handled automatically by the speak command).

ElevenLabs voices show friendly names (e.g., Sarah, Roger) and voice IDs.
Either can be used with --voice in the speak command.

OpenAI voices: alloy, ash, ballad, coral, echo, fable, nova, onyx, sage, shimmer, verse.
Default model: gpt-4o-mini-tts. Override with --model (e.g., tts-1-hd).

Examples:
  prompt-tools voices --language en-US --output table
  prompt-tools voices --language en-US --gender FEMALE
  prompt-tools voices --provider elevenlabs
  prompt-tools voices --provider elevenlabs --gender FEMALE --output table
  prompt-tools voices --provider openai --output table`,
	RunE: func(cmd *cobra.Command, args []string) error {
		providerName, _ := cmd.Flags().GetString("provider")
		language, _ := cmd.Flags().GetString("language")
		gender, _ := cmd.Flags().GetString("gender")

		if providerName == "" {
			cfg, err := appconfig.Load()
			if err != nil {
				return err
			}
			providerName = cfg.DefaultProvider
			if providerName == "" {
				providerName = "google"
			}
		}

		auth, err := resolveAuth(providerName)
		if err != nil {
			return err
		}

		tts, err := provider.NewTTS(providerName, auth)
		if err != nil {
			return err
		}

		voices, err := tts.ListVoices(language)
		if err != nil {
			return err
		}

		// Filter by gender if specified
		if gender != "" {
			gender = strings.ToUpper(gender)
			var filtered []provider.Voice
			for _, v := range voices {
				if strings.ToUpper(v.Gender) == gender {
					filtered = append(filtered, v)
				}
			}
			voices = filtered
		}

		if len(voices) == 0 {
			fmt.Fprintln(os.Stderr, "No voices found matching filters")
			return nil
		}

		return output.PrintObject(voices)
	},
}

func init() {
	voicesCmd.Flags().String("provider", "", "TTS provider (google, elevenlabs, openai)")
	voicesCmd.Flags().String("language", "", "Filter by language code (e.g., en-US)")
	voicesCmd.Flags().String("gender", "", "Filter by gender (MALE, FEMALE, NEUTRAL)")

	rootCmd.AddCommand(voicesCmd)
}

// resolveAuth resolves authentication for a provider.
// For Google: tries ADC (OAuth2) first, then API key (env var or keyring).
// For other providers: API key only (env var or keyring).
func resolveAuth(providerName string) (provider.AuthConfig, error) {
	var auth provider.AuthConfig

	// For Google, load project from config and try ADC/OAuth2 first
	if providerName == "google" {
		cfg, err := appconfig.Load()
		if err == nil {
			if gc, ok := cfg.Providers["google"]; ok {
				auth.Project = gc.ProjectID
			}
		}

		token, err := gcpauth.GetToken("https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return provider.AuthConfig{}, fmt.Errorf("GCP auth error: %w", err)
		}
		if token != nil {
			auth.BearerToken = token.AccessToken
			return auth, nil
		}
	}

	// Fall back to API key (env var, then keyring)
	envVars := map[string]string{
		"google":     "GOOGLE_API_KEY",
		"elevenlabs": "ELEVENLABS_API_KEY",
		"assemblyai": "ASSEMBLYAI_API_KEY",
		"openai":     "OPENAI_API_KEY",
	}
	if envVar, ok := envVars[providerName]; ok {
		if val := os.Getenv(envVar); val != "" {
			auth.APIKey = val
			return auth, nil
		}
	}

	// Fall back to keyring
	key, err := keyring.GetAPIKey(providerName)
	if err != nil {
		if providerName == "google" {
			return provider.AuthConfig{}, fmt.Errorf("no Google credentials found — either:\n  1. Run: gcloud auth application-default login (OAuth2, enables --style)\n  2. Set GOOGLE_API_KEY env var\n  3. Run: prompt-tools config set-api-key google")
		}
		return provider.AuthConfig{}, fmt.Errorf("no API key for %s — set via: prompt-tools config set-api-key %s (or env %s)", providerName, providerName, envVars[providerName])
	}
	auth.APIKey = key
	return auth, nil
}
