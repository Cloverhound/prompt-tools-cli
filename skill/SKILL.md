---
name: prompt-tools
description: "Prompt Tools CLI: generate IVR/contact center audio prompts via text-to-speech (Google Cloud TTS, Gemini TTS, ElevenLabs, OpenAI) and transcribe audio via speech-to-text (Google Cloud STT, AssemblyAI, OpenAI). Use for generating prompts, listing voices, bulk processing spreadsheets, and transcribing recordings."
argument-hint: "[command or resource-name]"
allowed-tools: Bash, Read, Grep, Glob
user-invocable: true
---

# Prompt Tools CLI Skill

This skill uses the `prompt-tools` CLI tool to generate IVR audio prompts and transcribe recordings.

**Setup:** Install via `curl -fsSL https://raw.githubusercontent.com/Cloverhound/prompt-tools-cli/main/install.sh | sh`, or build locally with `make build`.

**Binary path** (update to match your installation):
```bash
prompt-tools
```

## Authentication

### Google Cloud (two options)

**Option 1: GCP OAuth2/ADC (recommended)** — enables `--style` voice steering and server-side encoding:
```bash
gcloud auth application-default login   # One-time browser login
```
Also supports `GOOGLE_APPLICATION_CREDENTIALS` env var pointing to a service account JSON key file.

**Option 2: API Key** — simpler, works with all voice types except `--style`:
```bash
prompt-tools config set-api-key google   # Store in OS keyring
# or set GOOGLE_API_KEY env var
```

Google auth resolution order: ADC/OAuth2 > `GOOGLE_API_KEY` env var > OS keyring.

### Other Providers

API keys are stored in the OS keyring. Set them interactively:

```bash
prompt-tools setup                       # Interactive wizard (recommended for first run)
prompt-tools config set-api-key elevenlabs   # Store ElevenLabs API key
prompt-tools config set-api-key assemblyai   # Store AssemblyAI API key
prompt-tools config set-api-key openai       # Store OpenAI API key
prompt-tools config show                 # Show config and API key status
```

Or use environment variables: `ELEVENLABS_API_KEY`, `ASSEMBLYAI_API_KEY`, `OPENAI_API_KEY`.

Key resolution order: environment variable > OS keyring.

## Command Structure

```
prompt-tools setup                        # Interactive setup wizard
prompt-tools version                      # Print version
prompt-tools update                       # Self-update from GitHub releases
prompt-tools config <subcommand>          # Manage configuration
prompt-tools voices [flags]          # List available TTS voices
prompt-tools speak [text] [flags]         # Generate speech from text
prompt-tools bulk <subcommand> [flags]    # Bulk prompt generation
prompt-tools transcribe [flags]           # Transcribe a single audio file
prompt-tools batch-transcribe [flags]     # Batch transcribe multiple files
```

### Global Flags
- `--output json|table|csv|raw` — Output format (default: json)
- `--debug` — Show HTTP request/response details
- `--dry-run` — Show what would happen without executing

## Voice Types (Google)

Google TTS voices use two different APIs (same API key for both):

| Model | Quality | Example Voice Name | Google API |
|---|---|---|---|
| **Gemini** | Highest (bare names) | `Achernar`, `Kore`, `Puck` | Generative Language API |
| **Chirp3-HD** | High | `en-US-Chirp3-HD-Achernar` | Cloud Text-to-Speech API |
| **Chirp-HD** | High | `en-US-Chirp-HD-D` | Cloud Text-to-Speech API |
| **Studio** | High | `en-US-Studio-O` | Cloud Text-to-Speech API |
| **Neural2** | Good | `en-US-Neural2-F` | Cloud Text-to-Speech API |
| **Wavenet** | Good | `en-US-Wavenet-A` | Cloud Text-to-Speech API |
| **Standard** | Basic | `en-US-Standard-A` | Cloud Text-to-Speech API |

Gemini voices auto-select the best available model. Override with `--model`:
- `gemini-2.5-pro-preview-tts` — Highest quality (auto-selected default)
- `gemini-2.5-flash-preview-tts` — Fast, good quality

**Google Cloud setup requires enabling:**
1. [Cloud Text-to-Speech API](https://console.cloud.google.com/apis/library/texttospeech.googleapis.com) (for non-Gemini voices)
2. [Generative Language API](https://console.cloud.google.com/apis/library/generativelanguage.googleapis.com) (for Gemini voices)
3. [Cloud Speech-to-Text API](https://console.cloud.google.com/apis/library/speech.googleapis.com) (for transcription)

## Voice Types (ElevenLabs)

ElevenLabs voices can be specified by friendly name (e.g., `Sarah`, `Roger`) or voice ID. Names are resolved automatically via the API.

| Model | Quality | Notes |
|---|---|---|
| **eleven_v3** | Highest | Latest model (default) |
| **eleven_multilingual_v2** | High | Multilingual |
| **eleven_flash_v2_5** | Good | Fast, low latency |
| **eleven_turbo_v2_5** | Good | Low latency, multilingual |

Override model with `--model`. List voices with `prompt-tools voices --provider elevenlabs`.

**ElevenLabs API key requires these permissions:** Text to Speech > Access, Voices > Read, Models > Access.

## Voice Types (OpenAI)

OpenAI voices: alloy, ash, ballad, coral, echo, fable, nova, onyx, sage, shimmer, verse. All voices are multilingual.

| Model | Quality | Notes |
|---|---|---|
| **gpt-4o-mini-tts** | High | Default, most capable |
| **tts-1** | Standard | Lower latency |
| **tts-1-hd** | High | High definition |

Override model with `--model`. List voices with `prompt-tools voices --provider openai`.

## Speak Examples

```bash
# Simple text-to-speech (default: 8kHz mu-law WAV, Google Chirp3-HD)
prompt-tools speak "Welcome to customer support." -o welcome.wav

# Use a Gemini voice (highest quality)
prompt-tools speak "Welcome to customer support." --voice Achernar -o welcome.wav

# Use a specific Gemini model
prompt-tools speak "Hello" --voice Kore --model gemini-2.5-flash-preview-tts -o hello.wav

# SSML with pauses and emphasis
prompt-tools speak --ssml "<speak>Please hold.<break time='500ms'/>We'll be right with you.</speak>" -o hold.wav

# Read text from file
prompt-tools speak --file script.txt --voice en-US-Studio-O -o prompt.wav

# Wideband PCM output
prompt-tools speak "Hello" --sample-rate 16000 --encoding linear16 -o hello.wav

# MP3 output
prompt-tools speak "Hello" --format mp3 -o hello.mp3

# ElevenLabs provider (by voice name)
prompt-tools speak "Hello" --provider elevenlabs --voice Sarah -o hello.wav

# ElevenLabs with specific model
prompt-tools speak "Hello" --provider elevenlabs --voice Sarah --model eleven_multilingual_v2 -o hello.wav

# OpenAI provider
prompt-tools speak "Hello" --provider openai --voice alloy -o hello.wav

# OpenAI with model override
prompt-tools speak "Hello" --provider openai --voice nova --model tts-1-hd -o hello.wav

# Voice steering with --style (requires GCP OAuth2 auth)
prompt-tools speak "Hello world" --voice Achernar --style "speak warmly and professionally" -o hello.wav
prompt-tools speak "Welcome to support" --voice Kore --style "cheerful and upbeat tone" -o welcome.wav

# Language override for Gemini voices (default: en-US)
prompt-tools speak "Bonjour le monde" --voice Achernar --language fr-FR -o bonjour.wav
```

## Voice Listing Examples

```bash
# List all en-US voices (JSON)
prompt-tools voices --language en-US

# Table format
prompt-tools voices --language en-US --output table

# Filter by gender
prompt-tools voices --language en-US --gender FEMALE

# ElevenLabs voices
prompt-tools voices --provider elevenlabs

# OpenAI voices
prompt-tools voices --provider openai --output table
```

## Bulk Processing Examples

```bash
# Generate a blank template
prompt-tools bulk template --output prompts.xlsx
prompt-tools bulk template --output prompts.csv

# Validate a spreadsheet
prompt-tools bulk validate --file prompts.xlsx

# Generate all prompts from spreadsheet
prompt-tools bulk generate --file prompts.xlsx --output-dir ./output

# With overrides
prompt-tools bulk generate --file prompts.csv --output-dir ./output \
  --provider google --sample-rate 16000 --encoding linear16 --concurrency 10

# Skip existing files (incremental)
prompt-tools bulk generate --file prompts.xlsx --output-dir ./output --skip-existing

# Continue past errors
prompt-tools bulk generate --file prompts.xlsx --output-dir ./output --continue-on-error
```

### Bulk Template Format

| Filename | Voice | Text | SSML | Sample Rate | Encoding | Notes | Style | Language |
|---|---|---|---|---|---|---|---|---|
| welcome.wav | en-US-Chirp3-HD-Achernar | Welcome to support. | no | | | Main greeting | | |
| #holiday.wav | en-US-Chirp3-HD-Achernar | Closed for holiday. | no | | | Skipped (# prefix) | | |
| transfer.wav | Achernar | Hold please. | no | | | Gemini voice | speak warmly | en-US |
| es-MX/welcome.wav | es-MX-Neural2-A | Bienvenido. | no | | | Subdirectory | | |

- Row 1 = headers (skipped). Rows starting with `#` = skipped.
- Columns for Sample Rate, Encoding, Style, and Language are optional per-row overrides.
- Style: voice steering prompt (Gemini voices with GCP OAuth2 auth only).
- Language: language code for Gemini voices (default: en-US).
- Supports both `.xlsx` and `.csv` input (auto-detected by extension).
- Filename supports subdirectories (e.g., `en-US/welcome.wav`) — folders are created automatically under `--output-dir`.

## Transcription Examples

```bash
# Transcribe a single file (output to stdout)
prompt-tools transcribe --file recording.wav

# With timestamps
prompt-tools transcribe --file recording.wav --timestamps

# Save to file
prompt-tools transcribe --file recording.wav --output-file transcript.txt

# Boost IVR-specific phrases
prompt-tools transcribe --file recording.wav --phrases "IVR,UCCX,Webex"

# Use AssemblyAI
prompt-tools transcribe --file recording.wav --provider assemblyai

# Use OpenAI
prompt-tools transcribe --file recording.wav --provider openai

# Batch transcribe a directory
prompt-tools batch-transcribe --dir ./recordings --output-dir ./transcripts

# Batch with specific pattern
prompt-tools batch-transcribe --dir ./recordings --glob "*.mp3" --output-format csv

# Batch specific files
prompt-tools batch-transcribe --files "a.wav,b.wav,c.wav" --output-dir ./transcripts
```

## Configuration Examples

```bash
# Show full config
prompt-tools config show

# Set defaults
prompt-tools config set-provider google
prompt-tools config set-provider openai
prompt-tools config set-stt-provider assemblyai
prompt-tools config set-stt-provider openai
prompt-tools config set-voice en-US-Chirp3-HD-Achernar
prompt-tools config set-format wav
prompt-tools config set-sample-rate 8000
prompt-tools config set-encoding mulaw
prompt-tools config set-gcp-project my-project-id

# Manage API keys
prompt-tools config set-api-key google
prompt-tools config set-api-key elevenlabs
prompt-tools config set-api-key openai
prompt-tools config clear-api-key elevenlabs
```

## Audio Format Defaults

The default output is **8kHz mu-law WAV** — the universal IVR/telephony standard for North America. Common presets:

| Use Case | Sample Rate | Encoding | Flag Combo |
|---|---|---|---|
| North American IVR | 8000 | mulaw | (default) |
| European IVR | 8000 | alaw | `--encoding alaw` |
| Wideband/modern | 16000 | linear16 | `--sample-rate 16000 --encoding linear16` |
| General purpose | — | mp3 | `--format mp3` |

## When Answering Questions

1. **Check config first** with `prompt-tools config show` to see defaults and API key status
2. **Use `--dry-run`** to preview what a command would do before running it
3. **Use `--debug`** to see API requests when troubleshooting
4. **Check `--help`** on any command for full flag documentation
