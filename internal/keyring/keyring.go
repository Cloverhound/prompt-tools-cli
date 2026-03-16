package keyring

import (
	"fmt"

	gokeyring "github.com/zalando/go-keyring"
)

const serviceName = "prompt-tools-cli"

func SetAPIKey(provider, key string) error {
	return gokeyring.Set(serviceName, provider, key)
}

func GetAPIKey(provider string) (string, error) {
	key, err := gokeyring.Get(serviceName, provider)
	if err != nil {
		return "", fmt.Errorf("no API key found for %s: %w", provider, err)
	}
	return key, nil
}

func DeleteAPIKey(provider string) error {
	err := gokeyring.Delete(serviceName, provider)
	if err == gokeyring.ErrNotFound {
		return nil
	}
	return err
}
