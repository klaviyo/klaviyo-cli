// Package keyring stores account API keys in the OS keychain (macOS
// Keychain, Windows Credential Manager, Linux Secret Service).
//
// Keys are stored under the service name "klaviyo-cli" with the account
// profile name as the user. Note that KLAVIYO_CONFIG_DIR allows multiple
// config files to coexist, but they all share this one keychain namespace.
package keyring

import zkeyring "github.com/zalando/go-keyring"

const service = "klaviyo-cli"

// ErrNotFound is returned by Get when no key is stored for the profile.
var ErrNotFound = zkeyring.ErrNotFound

// Set stores the API key for the named account profile.
func Set(profile, apiKey string) error {
	return zkeyring.Set(service, profile, apiKey)
}

// Get returns the API key stored for the named account profile.
func Get(profile string) (string, error) {
	return zkeyring.Get(service, profile)
}

// Delete removes the key for the named account profile. Deleting a profile
// with no stored key is not an error.
func Delete(profile string) error {
	err := zkeyring.Delete(service, profile)
	if err == zkeyring.ErrNotFound {
		return nil
	}
	return err
}

// MockInit replaces the OS keychain with an in-memory store for tests.
func MockInit() { zkeyring.MockInit() }

// MockInitWithError makes every keychain operation fail with err, for
// testing unavailable-keychain paths.
func MockInitWithError(err error) { zkeyring.MockInitWithError(err) }
