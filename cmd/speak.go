package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cloverhound/prompt-tools-cli/internal/appconfig"
	"github.com/Cloverhound/prompt-tools-cli/internal/audio"
	"github.com/Cloverhound/prompt-tools-cli/internal/config"
	"github.com/Cloverhound/prompt-tools-cli/internal/provider"
	_ "github.com/Cloverhound/prompt-tools-cli/internal/tts"
	"github.com/spf13/cobra"
)

var speakCmd = &cobra.Command{
	Use:   "speak [text]",
	Short: "Generate speech from text",
	Long: `Generate an audio file from text or SSML using a TTS provider.

Text can be provided as a positional argument, via --text, --ssml, or --file.

Google voices use two naming styles:
  Structured:  en-US-Neural2-F, en-US-Chirp3-HD-Achernar, en-US-Studio-O
  Gemini:      Achernar, Kore, Puck (bare names — highest quality)

Gemini voices automatically use the Generative Language API. Override model with --model:
  gemini-2.5-pro-preview-tts    Highest quality (default for Gemini voices)
  gemini-2.5-flash-preview-tts  Fast, good quality

ElevenLabs voices can be specified by name (e.g., Sarah, Roger) or voice ID.
ElevenLabs models (override with --model):
  eleven_v3                     Latest, highest quality (default)
  eleven_multilingual_v2        High quality, multilingual
  eleven_flash_v2_5             Fast, low latency
  eleven_turbo_v2_5             Low latency, multilingual

Examples:
  prompt-tools speak "Hello world" -o hello.wav
  prompt-tools speak "Hello world" --voice Achernar -o hello.wav
  prompt-tools speak --ssml "<speak>Hello<break time='500ms'/>world</speak>" -o hello.wav
  prompt-tools speak --file script.txt --voice en-US-Studio-O -o prompt.wav
  prompt-tools speak "Hello" --voice Kore --model gemini-2.5-flash-preview-tts -o hello.wav
  prompt-tools speak --provider elevenlabs --voice Sarah -o hello.wav
  prompt-tools speak --provider elevenlabs --voice Sarah --model eleven_v3 -o hello.wav`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text, _ := cmd.Flags().GetString("text")
		ssml, _ := cmd.Flags().GetString("ssml")
		inputFile, _ := cmd.Flags().GetString("file")
		voice, _ := cmd.Flags().GetString("voice")
		providerName, _ := cmd.Flags().GetString("provider")
		outputPath, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")
		sampleRate, _ := cmd.Flags().GetInt("sample-rate")
		encoding, _ := cmd.Flags().GetString("encoding")
		speakingRate, _ := cmd.Flags().GetFloat64("speaking-rate")
		pitch, _ := cmd.Flags().GetFloat64("pitch")
		volumeGainDb, _ := cmd.Flags().GetFloat64("volume-gain-db")
		model, _ := cmd.Flags().GetString("model")

		// Positional arg as text
		if len(args) > 0 && text == "" && ssml == "" {
			text = args[0]
		}

		// Read from file if specified
		if inputFile != "" {
			data, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("reading input file: %w", err)
			}
			content := strings.TrimSpace(string(data))
			if strings.HasPrefix(content, "<speak>") {
				ssml = content
			} else {
				text = content
			}
		}

		// Validate input
		if text == "" && ssml == "" {
			return fmt.Errorf("provide text via argument, --text, --ssml, or --file")
		}
		if text != "" && ssml != "" {
			return fmt.Errorf("--text and --ssml are mutually exclusive")
		}
		if outputPath == "" {
			return fmt.Errorf("--output is required")
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
		if voice == "" {
			voice = cfg.DefaultVoice
			if voice == "" {
				voice = "en-US-Neural2-F"
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
				if format == "mp3" {
					encoding = "mp3"
				} else {
					encoding = "mulaw"
				}
			}
		}

		// Dry-run
		if config.DryRun() {
			fmt.Printf("Would synthesize with %s:\n", providerName)
			fmt.Printf("  Voice: %s\n", voice)
			fmt.Printf("  Format: %s\n", format)
			fmt.Printf("  Sample rate: %d Hz\n", sampleRate)
			fmt.Printf("  Encoding: %s\n", encoding)
			fmt.Printf("  Output: %s\n", outputPath)
			if text != "" {
				fmt.Printf("  Text: %s\n", text)
			} else {
				fmt.Printf("  SSML: %s\n", ssml)
			}
			return nil
		}

		// Resolve API key
		apiKey, err := resolveAPIKey(providerName)
		if err != nil {
			return err
		}

		tts, err := provider.NewTTS(providerName, apiKey)
		if err != nil {
			return err
		}

		// Synthesize
		req := &provider.TTSRequest{
			Text:         text,
			SSML:         ssml,
			Voice:        voice,
			Model:        model,
			SampleRate:   sampleRate,
			Encoding:     encoding,
			SpeakingRate: speakingRate,
			Pitch:        pitch,
			VolumeGainDb: volumeGainDb,
		}

		result, err := tts.Synthesize(req)
		if err != nil {
			return err
		}

		// Write output
		outputData := result.AudioData
		ext := strings.ToLower(filepath.Ext(outputPath))
		alreadyWAV := len(result.AudioData) >= 4 && string(result.AudioData[:4]) == "RIFF"
		if ext == ".wav" && result.Format != audio.EncodingMP3 && !alreadyWAV {
			outputData, err = audio.WriteWAV(result.AudioData, result.SampleRate, result.Format)
			if err != nil {
				return fmt.Errorf("writing WAV header: %w", err)
			}
		}

		if err := os.WriteFile(outputPath, outputData, 0644); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Generated %s (%d bytes)\n", outputPath, len(outputData))
		return nil
	},
}

func init() {
	speakCmd.Flags().String("text", "", "Plain text to synthesize")
	speakCmd.Flags().String("ssml", "", "SSML input (mutually exclusive with --text)")
	speakCmd.Flags().String("file", "", "Read text/SSML from file")
	speakCmd.Flags().String("voice", "", "Voice name")
	speakCmd.Flags().String("provider", "", "TTS provider (google, elevenlabs)")
	speakCmd.Flags().StringP("output", "o", "", "Output file (required)")
	speakCmd.Flags().String("format", "", "Audio format: wav, mp3")
	speakCmd.Flags().Int("sample-rate", 0, "Sample rate in Hz (8000, 16000, 22050, 24000)")
	speakCmd.Flags().String("encoding", "", "Audio encoding: mulaw, alaw, linear16, mp3")
	speakCmd.Flags().Float64("speaking-rate", 0, "Speaking rate multiplier")
	speakCmd.Flags().Float64("pitch", 0, "Pitch in semitones")
	speakCmd.Flags().Float64("volume-gain-db", 0, "Volume gain in dB")
	speakCmd.Flags().String("model", "", "TTS model (Gemini: gemini-2.5-pro-preview-tts; ElevenLabs: eleven_v3, eleven_multilingual_v2, eleven_flash_v2_5)")

	rootCmd.AddCommand(speakCmd)
}
