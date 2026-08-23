// Package auth stores API keys in the operating system keychain
// (macOS Keychain, Windows Credential Manager, Linux Secret Service).
//
// Keys are stored under the "klaviyo-cli" service, one entry per named
// account. On headless systems without a keychain, use the KLAVIYO_API_KEY
// environment variable instead of stored accounts.
package auth

import "github.com/zalando/go-keyring"

const service = "klaviyo-cli"

// SetKey stores the private API key for a named account.
func SetKey(account, key string) error {
	return keyring.Set(service, account, key)
}

// GetKey retrieves the private API key for a named account.
func GetKey(account string) (string, error) {
	return keyring.Get(service, account)
}

// DeleteKey removes the stored key for a named account.
func DeleteKey(account string) error {
	return keyring.Delete(service, account)
}
