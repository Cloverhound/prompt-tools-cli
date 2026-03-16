package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
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

	cfg, err := appconfig.Load()
	if err != nil {
		return err
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

	// Step 4: Default TTS provider
	if len(ttsProviders) > 1 {
		var defaultTTS string
		var ttsOpts []huh.Option[string]
		for _, p := range ttsProviders {
			label := p
			if p == "google" {
				label = "Google Cloud TTS"
			} else if p == "elevenlabs" {
				label = "ElevenLabs"
			}
			ttsOpts = append(ttsOpts, huh.NewOption(label, p))
		}
		defForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Default TTS provider").
					Options(ttsOpts...).
					Value(&defaultTTS),
			),
		)
		if err := defForm.Run(); err != nil {
			return err
		}
		cfg.DefaultProvider = defaultTTS
	} else {
		cfg.DefaultProvider = ttsProviders[0]
	}

	// Step 5: Default STT provider
	if len(sttProviders) > 1 {
		var defaultSTT string
		var sttOpts []huh.Option[string]
		for _, p := range sttProviders {
			label := p
			if p == "google" {
				label = "Google Cloud STT"
			} else if p == "assemblyai" {
				label = "AssemblyAI"
			}
			sttOpts = append(sttOpts, huh.NewOption(label, p))
		}
		defForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Default STT provider").
					Options(sttOpts...).
					Value(&defaultSTT),
			),
		)
		if err := defForm.Run(); err != nil {
			return err
		}
		cfg.DefaultSTTProvider = defaultSTT
	} else {
		cfg.DefaultSTTProvider = sttProviders[0]
	}

	// Step 6: Default audio format
	var audioPreset string
	audioForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default audio format for IVR prompts").
				Options(
					huh.NewOption("8kHz mu-law WAV  (Standard North American telephony)", "mulaw-8k"),
					huh.NewOption("8kHz A-law WAV   (Standard European telephony)", "alaw-8k"),
					huh.NewOption("16kHz PCM WAV    (Wideband / modern systems)", "pcm-16k"),
					huh.NewOption("MP3              (General purpose)", "mp3"),
				).
				Value(&audioPreset),
		),
	)
	if err := audioForm.Run(); err != nil {
		return err
	}

	switch audioPreset {
	case "mulaw-8k":
		cfg.DefaultFormat = "wav"
		cfg.DefaultSampleRate = 8000
		cfg.DefaultEncoding = "mulaw"
	case "alaw-8k":
		cfg.DefaultFormat = "wav"
		cfg.DefaultSampleRate = 8000
		cfg.DefaultEncoding = "alaw"
	case "pcm-16k":
		cfg.DefaultFormat = "wav"
		cfg.DefaultSampleRate = 16000
		cfg.DefaultEncoding = "linear16"
	case "mp3":
		cfg.DefaultFormat = "mp3"
		cfg.DefaultSampleRate = 24000
		cfg.DefaultEncoding = "mp3"
	}

	// Set default voice
	cfg.DefaultVoice = "en-US-Neural2-F"

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\n  ✓ Configuration saved to %s\n", appconfig.ConfigDir()+"/config.json")
	fmt.Fprintf(os.Stderr, "  ✓ API keys stored in system keyring\n")
	fmt.Fprintf(os.Stderr, "\n  You're all set! Try: prompt-tools speak \"Hello world\" --output hello.wav\n\n")

	return nil
}
