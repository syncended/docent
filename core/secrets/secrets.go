// Package secrets keeps API keys in the OS credential store (Keychain,
// Windows Credential Manager, Secret Service).
package secrets

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const service = "docent"

// APIKey returns the stored key for an agent provider, or "" if there is none.
func APIKey(provider string) (string, error) {
	key, err := keyring.Get(service, provider)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return key, err
}

// SetAPIKey stores the key for an agent provider.
func SetAPIKey(provider, key string) error {
	return keyring.Set(service, provider, key)
}

// DeleteAPIKey removes the key for an agent provider, if any.
func DeleteAPIKey(provider string) error {
	err := keyring.Delete(service, provider)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
