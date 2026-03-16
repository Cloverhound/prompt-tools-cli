# Prompt Tools CLI

A command-line tool for IVR/contact center prompt generation (text-to-speech) and audio transcription (speech-to-text).

## Install

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/Cloverhound/prompt-tools-cli/main/install.sh | sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/Cloverhound/prompt-tools-cli/main/install.ps1 | iex
```

Or download from [Releases](https://github.com/Cloverhound/prompt-tools-cli/releases).

## Getting API Keys

You need at least one TTS provider key to generate prompts, and one STT provider key to transcribe audio.

### Google Cloud (TTS + STT)

One API key covers both Text-to-Speech and Speech-to-Text.

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a project or select an existing one
3. Enable the [Cloud Text-to-Speech API](https://console.cloud.google.com/apis/library/texttospeech.googleapis.com)
4. Enable the [Cloud Speech-to-Text API](https://console.cloud.google.com/apis/library/speech.googleapis.com)
5. Go to [APIs & Services > Credentials](https://console.cloud.google.com/apis/credentials)
6. Click **Create Credentials > API Key**
7. Copy the key

New accounts get $300 in free credits. TTS pricing is ~$4 per 1M characters for Neural2 voices, ~$16/1M for Gemini/Chirp3-HD.

### ElevenLabs (TTS)

1. Sign up at [elevenlabs.io](https://elevenlabs.io/)
2. Go to [Profile + API Key](https://elevenlabs.io/app/settings/api-keys)
3. Copy your API key

Free tier includes limited characters per month. Paid plans start at $5/mo.

### AssemblyAI (STT)

1. Sign up at [assemblyai.com](https://www.assemblyai.com/)
2. Go to your [Dashboard](https://www.assemblyai.com/app)
3. Copy your API key from the sidebar

Free tier includes transcription hours. Pay-as-you-go at $0.37/hour after that.

### Store Your Keys

```bash
# Interactive setup (recommended for first run)
prompt-tools setup

# Or set keys individually
prompt-tools config set-api-key google
prompt-tools config set-api-key elevenlabs
prompt-tools config set-api-key assemblyai
```

Keys are stored in your OS keyring (macOS Keychain / Linux keyring / Windows Credential Manager), never in plain text files.

## Quick Start

```bash
# Interactive setup (API keys, defaults)
prompt-tools setup

# Generate a prompt
prompt-tools speak "Welcome to customer support." -o welcome.wav

# Use a Gemini voice (highest quality)
prompt-tools speak "Welcome to customer support." --voice Achernar -o welcome.wav

# List available voices
prompt-tools voices --language en-US --output table

# Bulk generate from spreadsheet
prompt-tools bulk template --output prompts.xlsx    # Create template
prompt-tools bulk generate --file prompts.xlsx --output-dir ./output

# Transcribe audio
prompt-tools transcribe --file recording.wav
```

## TTS Providers

### Google Cloud TTS

400+ voices across multiple model families. Default provider.

| Model | Quality | Example Voice | Notes |
|-------|---------|---------------|-------|
| Gemini | Highest | `Achernar`, `Kore`, `Puck` | Bare names, uses Gemini 2.5 Pro |
| Chirp3-HD | High | `en-US-Chirp3-HD-Achernar` | Same voices, different model |
| Studio | High | `en-US-Studio-O` | Studio-grade |
| Neural2 | Good | `en-US-Neural2-F` | Good default |
| Wavenet | Good | `en-US-Wavenet-A` | DeepMind Wavenet |
| Standard | Basic | `en-US-Standard-A` | Concatenative |

Gemini voices automatically use `gemini-2.5-pro-tts`. Override with `--model`:

```bash
prompt-tools speak "Hello" --voice Kore --model gemini-2.5-flash-tts -o hello.wav
```

### ElevenLabs

Premium natural voices. Output is converted to IVR-compatible formats (mu-law/A-law WAV) automatically.

```bash
prompt-tools speak "Hello" --provider elevenlabs --voice <voice-id> -o hello.wav
```

## STT Providers

### Google Cloud STT

Sync recognition for short audio, phrase boosting, word-level timestamps.

### AssemblyAI

Async transcription with polling, high accuracy, automatic punctuation.

```bash
prompt-tools transcribe --file recording.wav --provider assemblyai
```

## Bulk Processing

Generate hundreds of prompts from a spreadsheet. Supports `.xlsx` and `.csv`.

```bash
# Create a template
prompt-tools bulk template --output prompts.xlsx

# Validate without generating
prompt-tools bulk validate --file prompts.xlsx

# Generate all prompts
prompt-tools bulk generate --file prompts.xlsx --output-dir ./output

# With options
prompt-tools bulk generate --file prompts.csv --output-dir ./output \
  --concurrency 10 --skip-existing --continue-on-error
```

### Spreadsheet Format

| Filename | Voice | Text | SSML | Sample Rate | Encoding | Notes |
|----------|-------|------|------|-------------|----------|-------|
| welcome.wav | en-US-Neural2-F | Welcome to support. | no | | | Main greeting |
| es-MX/welcome.wav | es-MX-Neural2-A | Bienvenido. | no | | | Subdirectory |
| transfer.wav | Achernar | Hold please. | no | | | Gemini voice |
| #holiday.wav | en-US-Neural2-F | Closed for holiday. | no | | | Skipped |

- Rows starting with `#` are skipped
- Voice, Sample Rate, and Encoding are optional (defaults from config)
- Filename supports subdirectories — folders are created automatically

## Batch Transcription

```bash
# Transcribe a directory
prompt-tools batch-transcribe --dir ./recordings --output-dir ./transcripts

# Specific files
prompt-tools batch-transcribe --files "a.wav,b.wav" --output-format csv

# With concurrency
prompt-tools batch-transcribe --dir ./recordings --concurrency 10 --continue-on-error
```

## Authentication

API keys are stored in the OS keyring (macOS Keychain / Linux keyring / Windows Credential Manager).

```bash
prompt-tools setup                          # Interactive wizard
prompt-tools config set-api-key google      # Set Google API key
prompt-tools config set-api-key elevenlabs  # Set ElevenLabs API key
prompt-tools config set-api-key assemblyai  # Set AssemblyAI API key
prompt-tools config clear-api-key google    # Remove a key
prompt-tools config show                    # Show config and key status
```

Key resolution order: environment variable (`GOOGLE_API_KEY`, `ELEVENLABS_API_KEY`, `ASSEMBLYAI_API_KEY`) > OS keyring.

## Audio Formats

Default output is **8kHz mu-law WAV** — the North American IVR/telephony standard.

| Use Case | Sample Rate | Encoding | Flags |
|----------|-------------|----------|-------|
| North American IVR | 8000 | mulaw | (default) |
| European IVR | 8000 | alaw | `--encoding alaw` |
| Wideband / modern | 16000 | linear16 | `--sample-rate 16000 --encoding linear16` |
| General purpose | — | mp3 | `--format mp3` |

```bash
prompt-tools config set-sample-rate 8000
prompt-tools config set-encoding mulaw
prompt-tools config set-format wav
```

## Output Formats

Control output with `--output`:

| Format | Description |
|--------|-------------|
| `json` | Pretty-printed JSON (default) |
| `table` | ASCII table with terminal-width formatting |
| `csv` | CSV with headers |
| `raw` | Raw output |

```bash
prompt-tools voices --language en-US --output table
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--output json\|table\|csv\|raw` | Output format (default: json) |
| `--debug` | Show HTTP request/response details |
| `--dry-run` | Show plan without executing |

## Shell Completions

```bash
# Zsh
prompt-tools completion zsh > "${fpath[1]}/_prompt-tools"

# Bash
prompt-tools completion bash > /etc/bash_completion.d/prompt-tools

# Fish
prompt-tools completion fish > ~/.config/fish/completions/prompt-tools.fish
```

## Claude Code Integration

A Claude Code skill is included in `skill/SKILL.md`. To install:

```bash
mkdir -p ~/.claude/skills/prompt-tools
cp skill/SKILL.md ~/.claude/skills/prompt-tools/SKILL.md
```

## Development

See [CLAUDE.md](CLAUDE.md) for project structure and conventions.

```bash
make build    # Build binary
make check    # Build + go vet
go test ./... # Run tests
```

## License

MIT
