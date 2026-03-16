package provider

import "fmt"

var (
	ttsProviders = make(map[string]func(apiKey string) TTSProvider)
	sttProviders = make(map[string]func(apiKey string) STTProvider)
)

func RegisterTTS(name string, factory func(apiKey string) TTSProvider) {
	ttsProviders[name] = factory
}

func RegisterSTT(name string, factory func(apiKey string) STTProvider) {
	sttProviders[name] = factory
}

func NewTTS(name, apiKey string) (TTSProvider, error) {
	factory, ok := ttsProviders[name]
	if !ok {
		return nil, fmt.Errorf("unknown TTS provider: %s", name)
	}
	return factory(apiKey), nil
}

func NewSTT(name, apiKey string) (STTProvider, error) {
	factory, ok := sttProviders[name]
	if !ok {
		return nil, fmt.Errorf("unknown STT provider: %s", name)
	}
	return factory(apiKey), nil
}
