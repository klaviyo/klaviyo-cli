package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckForUpdate(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
	}))
	defer srv.Close()
	oldURL := releaseURL
	releaseURL = srv.URL
	defer func() { releaseURL = oldURL }()

	if got := checkForUpdate("0.1.0"); got != "v0.2.0" {
		t.Errorf("newer release: got %q, want v0.2.0", got)
	}
	if got := checkForUpdate("0.2.0"); got != "" {
		t.Errorf("up to date: got %q, want empty", got)
	}
	if got := checkForUpdate("0.3.0"); got != "" {
		t.Errorf("ahead of latest: got %q, want empty", got)
	}
	if hits != 1 {
		t.Errorf("network hits = %d, want 1 (later checks must use the 24h cache)", hits)
	}
}

func TestCheckForUpdateSurvivesNetworkFailure(t *testing.T) {
	t.Setenv("KLAVIYO_CONFIG_DIR", t.TempDir())
	oldURL := releaseURL
	releaseURL = "http://127.0.0.1:1/unreachable"
	defer func() { releaseURL = oldURL }()

	if got := checkForUpdate("0.1.0"); got != "" {
		t.Errorf("got %q, want empty on network failure", got)
	}
}

func TestShouldCheckForUpdateRespectsOptOut(t *testing.T) {
	t.Setenv("KLAVIYO_NO_UPDATE_NOTIFIER", "1")
	if shouldCheckForUpdate() {
		t.Error("should not check when KLAVIYO_NO_UPDATE_NOTIFIER is set")
	}
}
