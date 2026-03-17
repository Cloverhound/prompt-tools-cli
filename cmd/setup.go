package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/keyring"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard (first-run experience)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetup()
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup() error {
	fmt.Print("\n  Welcome to Prompt Tools! Let's get you set up.\n\n")

	// Step 1: Select TTS providers
	var ttsProviders []string
	ttsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which text-to-speech providers would you like to use?").
				Options(
					huh.NewOption("Google Cloud TTS  (400+ voices, native IVR formats, SSML support)", "google"),
					huh.NewOption("ElevenLabs        (Premium natural voices, requires format conversion)", "elevenlabs"),
					huh.NewOption("OpenAI            (High quality natural voices, simple API)", "openai"),
				).
				Value(&ttsProviders),
		),
	)
	if err := ttsForm.Run(); err != nil {
		return err
	}
	if len(ttsProviders) == 0 {
		return fmt.Errorf("at least one TTS provider must be selected")
	}

	// Step 2: Select STT providers
	var sttProviders []string
	sttForm := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which speech-to-text providers would you like to use?").
				Options(
					huh.NewOption("Google Cloud STT  (Phrase boosting, word timestamps, long audio support)", "google"),
					huh.NewOption("AssemblyAI        (High accuracy, simple API, automatic punctuation)", "assemblyai"),
					huh.NewOption("OpenAI            (Whisper & GPT-4o transcription, sync API)", "openai"),
				).
				Value(&sttProviders),
		),
	)
	if err := sttForm.Run(); err != nil {
		return err
	}
	if len(sttProviders) == 0 {
		return fmt.Errorf("at least one STT provider must be selected")
	}

	// Step 3: Collect API keys for selected providers
	// Determine unique providers needing keys
	needsKey := make(map[string]bool)
	for _, p := range ttsProviders {
		needsKey[p] = true
	}
	for _, p := range sttProviders {
		needsKey[p] = true
	}

	// Google key (shared for TTS and STT)
	if needsKey["google"] {
		fmt.Println("\n  --- Google Cloud TTS & STT ---")
		var googleKey string
		keyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Google Cloud API Key").
					EchoMode(huh.EchoModePassword).
					Value(&googleKey),
			),
		)
		if err := keyForm.Run(); err != nil {
			return err
		}
		googleKey = strings.TrimSpace(googleKey)
		if googleKey != "" {
			if err := keyring.SetAPIKey("google", googleKey); err != nil {
				return fmt.Errorf("saving Google API key: %w", err)
			}
			fmt.Println("  ✓ API key saved to system keyring")
		}

	}

	// ElevenLabs key
	if needsKey["elevenlabs"] {
		fmt.Println("\n  --- ElevenLabs ---")
		var elKey string
		keyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("ElevenLabs API Key").
					EchoMode(huh.EchoModePassword).
					Value(&elKey),
			),
		)
		if err := keyForm.Run(); err != nil {
			return err
		}
		elKey = strings.TrimSpace(elKey)
		if elKey != "" {
			if err := keyring.SetAPIKey("elevenlabs", elKey); err != nil {
				return fmt.Errorf("saving ElevenLabs API key: %w", err)
			}
			fmt.Println("  ✓ API key saved to system keyring")
		}
	}

	// AssemblyAI key
	if needsKey["assemblyai"] {
		fmt.Println("\n  --- AssemblyAI ---")
		var aaiKey string
		keyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("AssemblyAI API Key").
					EchoMode(huh.EchoModePassword).
					Value(&aaiKey),
			),
		)
		if err := keyForm.Run(); err != nil {
			return err
		}
		aaiKey = strings.TrimSpace(aaiKey)
		if aaiKey != "" {
			if err := keyring.SetAPIKey("assemblyai", aaiKey); err != nil {
				return fmt.Errorf("saving AssemblyAI API key: %w", err)
			}
			fmt.Println("  ✓ API key saved to system keyring")
		}
	}

	// OpenAI key
	if needsKey["openai"] {
		fmt.Println("\n  --- OpenAI ---")
		var oaiKey string
		keyForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("OpenAI API Key").
					EchoMode(huh.EchoModePassword).
					Value(&oaiKey),
			),
		)
		if err := keyForm.Run(); err != nil {
			return err
		}
		oaiKey = strings.TrimSpace(oaiKey)
		if oaiKey != "" {
			if err := keyring.SetAPIKey("openai", oaiKey); err != nil {
				return fmt.Errorf("saving OpenAI API key: %w", err)
			}
			fmt.Println("  ✓ API key saved to system keyring")
		}
	}

	fmt.Fprintf(os.Stderr, "\n  ✓ API keys stored in system keyring\n")
	fmt.Fprintf(os.Stderr, "\n  You're all set! Try: prompt-tools speak \"Hello world\" --output hello.wav\n")
	fmt.Fprintf(os.Stderr, "  Customize defaults with: prompt-tools config set-provider, set-voice, set-format, etc.\n\n")

	return nil
}
