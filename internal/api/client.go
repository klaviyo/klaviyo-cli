// Package api provides a minimal HTTP client for the Klaviyo API.
//
// It handles authentication headers, the API revision header, and retries
// for rate-limited (429) and transient server errors.
package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultRevision (see revision_gen.go) is generated from the vendored
// OpenAPI spec; override per request with --revision.
const (
	defaultBaseURL = "https://a.klaviyo.com"
	maxAttempts    = 4
)

// Client is an authenticated Klaviyo API client bound to one account's key.
type Client struct {
	BaseURL  string
	Revision string

	apiKey    string
	userAgent string
	http      *http.Client
	// sleep is stubbed in tests to avoid real backoff waits.
	sleep func(time.Duration)
}

// NewClient returns a client using the given private API key and revision.
// cliVersion is embedded in the User-Agent header.
func NewClient(apiKey, revision, cliVersion string) *Client {
	if revision == "" {
		revision = DefaultRevision
	}
	return &Client{
		BaseURL:   defaultBaseURL,
		Revision:  revision,
		apiKey:    apiKey,
		userAgent: "klaviyo-cli/" + cliVersion,
		http:      &http.Client{Timeout: 60 * time.Second},
		sleep:     time.Sleep,
	}
}

// Response is a completed API response with its body fully read.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// OK reports whether the response status is in the 2xx range.
func (r *Response) OK() bool { return r.StatusCode >= 200 && r.StatusCode < 300 }

// Do performs an API request and returns the response.
//
// path may be given with or without a leading slash (e.g. "api/profiles").
// 429 responses are retried honoring Retry-After; 5xx responses are retried
// only for idempotent methods (GET/HEAD). A non-2xx final response is
// returned without error so callers can render the API's error body.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body []byte) (*Response, error) {
	u, err := c.buildURL(path, query)
	if err != nil {
		return nil, err
	}

	var resp *Response
	for attempt := 1; ; attempt++ {
		resp, err = c.doOnce(ctx, method, u, body)
		if err != nil {
			return nil, err
		}
		if attempt >= maxAttempts || !c.shouldRetry(method, resp.StatusCode) {
			return resp, nil
		}
		c.sleep(retryDelay(resp, attempt))
	}
}

func (c *Client) buildURL(path string, query url.Values) (string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", path, err)
	}
	if len(query) > 0 {
		q := u.Query()
		for k, vs := range query {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func (c *Client) doOnce(ctx context.Context, method, u string, body []byte) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Klaviyo-API-Key "+c.apiKey)
	req.Header.Set("revision", c.Revision)
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/vnd.api+json")
	}

	httpResp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(httpResp.Body); err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return &Response{
		StatusCode: httpResp.StatusCode,
		Header:     httpResp.Header,
		Body:       buf.Bytes(),
	}, nil
}

func (c *Client) shouldRetry(method string, status int) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	if status >= 500 && (method == http.MethodGet || method == http.MethodHead) {
		return true
	}
	return false
}

func retryDelay(resp *Response, attempt int) time.Duration {
	if s := resp.Header.Get("Retry-After"); s != "" {
		if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	// Exponential backoff: 1s, 2s, 4s.
	return time.Duration(1<<(attempt-1)) * time.Second
}
