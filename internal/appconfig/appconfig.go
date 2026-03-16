package appconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ProviderConfig struct {
	ProjectID string `json:"project_id,omitempty"`
}

type Config struct {
	DefaultProvider    string                    `json:"default_provider"`
	DefaultVoice       string                    `json:"default_voice"`
	DefaultSampleRate  int                       `json:"default_sample_rate"`
	DefaultEncoding    string                    `json:"default_encoding"`
	DefaultFormat      string                    `json:"default_format"`
	DefaultSTTProvider string                    `json:"default_stt_provider"`
	Providers          map[string]ProviderConfig `json:"providers,omitempty"`

	path string // not serialized
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".prompt-tools")
}

func configPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

func Load() (*Config, error) {
	cfg := &Config{
		Providers: make(map[string]ProviderConfig),
		path:      configPath(),
	}

	data, err := os.ReadFile(cfg.path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}
	return cfg, nil
}

func (c *Config) Save() error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// Defaults returns a config with IVR-standard defaults applied
func Defaults() *Config {
	return &Config{
		DefaultProvider:    "google",
		DefaultVoice:       "en-US-Neural2-F",
		DefaultSampleRate:  8000,
		DefaultEncoding:    "mulaw",
		DefaultFormat:      "wav",
		DefaultSTTProvider: "google",
		Providers:          make(map[string]ProviderConfig),
	}
}
