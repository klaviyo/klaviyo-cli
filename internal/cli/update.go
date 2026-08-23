package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/term"

	"github.com/klaviyo/klaviyo-cli/internal/config"
)

// The update notifier (modeled on gh and the Stripe CLI): check GitHub for a
// newer release at most once per 24h, in the background, and print a notice
// to stderr after the command finishes. Never blocks meaningfully (short HTTP
// timeout), never runs for dev builds, in CI, when piped, or when opted out
// via KLAVIYO_NO_UPDATE_NOTIFIER. The CLI does not self-update; releases
// install through the original channel.

const updateCheckInterval = 24 * time.Hour

// Overridden in tests.
var releaseURL = "https://api.github.com/repos/klaviyo/klaviyo-cli/releases/latest"

type updateState struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

func shouldCheckForUpdate() bool {
	if version == "dev" {
		return false
	}
	if os.Getenv("KLAVIYO_NO_UPDATE_NOTIFIER") != "" || os.Getenv("CI") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// startUpdateCheck runs the check in the background; the channel receives the
// newer version string, or "" when up to date or on any error.
func startUpdateCheck() <-chan string {
	ch := make(chan string, 1)
	go func() { ch <- checkForUpdate(version) }()
	return ch
}

func maybeNotifyUpdate(ch <-chan string) {
	if latest := <-ch; latest != "" {
		fmt.Fprintf(os.Stderr, "\nA new release of klaviyo is available: %s → %s\n"+
			"https://github.com/klaviyo/klaviyo-cli/releases/latest\n", version, latest)
	}
}

// checkForUpdate returns the latest version when it is newer than current,
// using a 24h on-disk cache to avoid hitting the network on every run.
// All failures return "" — a broken check must never break the CLI.
func checkForUpdate(current string) string {
	state, statePath := loadUpdateState()
	latest := state.Latest
	if time.Since(state.CheckedAt) >= updateCheckInterval {
		fetched, err := fetchLatestVersion()
		if err != nil {
			return ""
		}
		latest = fetched
		saveUpdateState(statePath, updateState{CheckedAt: time.Now(), Latest: latest})
	}
	if latest != "" && semver.Compare(canonical(latest), canonical(current)) > 0 {
		return latest
	}
	return ""
}

func canonical(v string) string {
	return "v" + strings.TrimPrefix(v, "v")
}

func fetchLatestVersion() (string, error) {
	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "klaviyo-cli/"+version)
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	// The tag is remote data printed to a terminal. A valid semver cannot
	// carry control characters, so this check doubles as escape-injection
	// protection for the stderr notice — keep it even if comparison logic
	// changes.
	if !semver.IsValid(canonical(release.TagName)) {
		return "", fmt.Errorf("malformed release tag %q", canonical(release.TagName))
	}
	return release.TagName, nil
}

func loadUpdateState() (updateState, string) {
	var state updateState
	dir, err := config.Dir()
	if err != nil {
		return state, ""
	}
	path := filepath.Join(dir, "update-check.json")
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &state)
	}
	return state, path
}

func saveUpdateState(path string, state updateState) {
	if path == "" {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, data, 0o600)
}
