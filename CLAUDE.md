# Prompt Tools CLI

IVR/contact center prompt generation CLI tool. Go + Cobra, following the same conventions as webex-cli.

## Build & Test

```bash
make build          # Build binary
make check          # Build + go vet
go test ./...       # Run tests
go vet ./...        # Static analysis
```

## Project Structure

- `cmd/` — Cobra command definitions (root, config, speak, voices, bulk, transcribe, etc.)
- `internal/appconfig/` — Config file management (~/.prompt-tools/config.json)
- `internal/config/` — Runtime state (debug, dry-run flags)
- `internal/output/` — JSON/table/CSV/raw output formatting
- `internal/keyring/` — API key storage in OS keyring
- `internal/provider/` — TTSProvider and STTProvider interfaces, registry, AuthConfig
- `internal/gcpauth/` — GCP Application Default Credentials resolution and token exchange (no SDK)
- `internal/tts/` — TTS provider implementations (Google, ElevenLabs, OpenAI)
- `internal/stt/` — STT provider implementations (Google, AssemblyAI, OpenAI)
- `internal/audio/` — WAV header generation, PCM/mulaw/alaw conversion
- `internal/bulk/` — Spreadsheet parsing, template generation, concurrent pipeline

## Key Conventions

- REST over SDK: Use net/http directly, no Google Cloud SDK
- Auth resolution (Google): ADC/OAuth2 → env var → OS keyring; (others): env var → OS keyring
- Default audio: 8kHz mu-law WAV (IVR standard)
- Provider registration: init() functions in tts/ and stt/ packages register with provider.Registry
- Output formatting: Use output.PrintObject() for structured data, fmt.Fprintf(os.Stderr, ...) for status messages
- Config: Non-secret config in ~/.prompt-tools/config.json, secrets in OS keyring under "prompt-tools-cli" service name

## Environment Variables

- `GOOGLE_APPLICATION_CREDENTIALS` — Path to GCP service account JSON key file (ADC)
- `GOOGLE_API_KEY` — Google Cloud API key
- `ELEVENLABS_API_KEY` — ElevenLabs API key
- `ASSEMBLYAI_API_KEY` — AssemblyAI API key
- `OPENAI_API_KEY` — OpenAI API key

## UI Change Checklist

When changing commands, flags, help text, or user-facing behavior, update ALL of these:

1. **Command help text** — `Long` and `Short` fields in the cobra.Command (in `cmd/*.go`)
2. **`skill/SKILL.md`** — The Claude Code skill reference (command examples, flag docs)
3. **`~/.claude/skills/prompt-tools/SKILL.md`** — Installed copy of the skill (copy from repo after editing)
4. **`CLAUDE.md`** — This file, if conventions or structure changed
5. **`cmd/setup.go`** — If providers, defaults, or audio presets changed
