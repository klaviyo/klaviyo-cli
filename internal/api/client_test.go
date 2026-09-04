package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestClient(baseURL string) *Client {
	c := NewClient("pk_test", "2026-07-15", "test")
	c.BaseURL = baseURL
	c.sleep = func(time.Duration) {}
	return c
}

func TestDoSetsHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).Do(context.Background(), "GET", "api/accounts/", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK() {
		t.Fatalf("status = %d, want 2xx", resp.StatusCode)
	}
	if auth := got.Get("Authorization"); auth != "Klaviyo-API-Key pk_test" {
		t.Errorf("Authorization = %q", auth)
	}
	if rev := got.Get("revision"); rev != "2026-07-15" {
		t.Errorf("revision = %q", rev)
	}
}

func TestDoRetriesOn429(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).Do(context.Background(), "POST", "/api/events/", nil, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
	if !resp.OK() {
		t.Errorf("status = %d, want 2xx", resp.StatusCode)
	}
}

func TestDoDoesNotRetry5xxOnPost(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	resp, err := newTestClient(srv.URL).Do(context.Background(), "POST", "/api/events/", nil, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (POST must not retry 5xx)", attempts)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestDoMergesQueryParams(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	q := url.Values{}
	q.Set("filter", `equals(email,"a@b.com")`)
	if _, err := newTestClient(srv.URL).Do(context.Background(), "GET", "/api/profiles/", q, nil); err != nil {
		t.Fatal(err)
	}
	if gotQuery.Get("filter") != `equals(email,"a@b.com")` {
		t.Errorf("filter = %q", gotQuery.Get("filter"))
	}
}

func TestVerboseLogsRequestAndResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("RateLimit-Remaining", "42")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient("pk_test", "", "test")
	c.BaseURL = srv.URL
	var log bytes.Buffer
	c.Verbose = &log

	if _, err := c.Do(context.Background(), "GET", "/api/metrics/", nil, nil); err != nil {
		t.Fatal(err)
	}
	s := log.String()
	for _, want := range []string{
		"> GET " + srv.URL + "/api/metrics/",
		"revision " + DefaultRevision,
		"< HTTP 200",
		"Ratelimit-Remaining: 42",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("verbose log missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "pk_test") {
		t.Error("verbose log must not contain the API key")
	}
	// net/http rejects control bytes in headers before they reach us, so
	// scrubControl is defense-in-depth; test it directly.
	if got := scrubControl("a\x1b[31mb"); got != "a.[31mb" {
		t.Errorf("scrubControl = %q", got)
	}
}
