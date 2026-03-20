package provider

import "fmt"

var (
	ttsProviders = make(map[string]func(auth AuthConfig) TTSProvider)
	sttProviders = make(map[string]func(auth AuthConfig) STTProvider)
)

func RegisterTTS(name string, factory func(auth AuthConfig) TTSProvider) {
	ttsProviders[name] = factory
}

func RegisterSTT(name string, factory func(auth AuthConfig) STTProvider) {
	sttProviders[name] = factory
}

func NewTTS(name string, auth AuthConfig) (TTSProvider, error) {
	factory, ok := ttsProviders[name]
	if !ok {
		return nil, fmt.Errorf("unknown TTS provider: %s", name)
	}
	return factory(auth), nil
}

func NewSTT(name string, auth AuthConfig) (STTProvider, error) {
	factory, ok := sttProviders[name]
	if !ok {
		return nil, fmt.Errorf("unknown STT provider: %s", name)
	}
	return factory(auth), nil
}
