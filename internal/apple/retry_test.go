// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const apple500 = `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred.","detail":"An unexpected error occurred on the server side."}]}`

// newRetryTestClient is NewClient's pair of clients with millisecond waits, so
// a test exercises the real policies without sleeping through the backoff.
func newRetryTestClient(srv *httptest.Server, maxRetries int, timeout time.Duration) *Client {
	return &Client{
		HostURL:         srv.URL,
		HTTPClient:      newRetryingClient(timeout, maxRetries, readRetryPolicy, time.Millisecond, 5*time.Millisecond),
		WriteHTTPClient: newRetryingClient(timeout, maxRetries, writeRetryPolicy, time.Millisecond, 5*time.Millisecond),
		Token:           "test",
	}
}

// scripted answers each request with the next status in statuses, repeating
// the last one, and counts the requests.
func scripted(t *testing.T, calls *atomic.Int32, statuses ...int) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1))
		status := statuses[min(n, len(statuses))-1]
		w.Header().Set("Content-Type", "application/json")
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "0")
		}
		w.WriteHeader(status)
		switch {
		case status >= 500:
			_, _ = w.Write([]byte(apple500))
		case status < 300:
			_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"A1"}}`))
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func do(t *testing.T, c *Client, method string, body string) ([]byte, error) {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, c.HostURL+"/v1/apps/A1", reader)
	if err != nil {
		t.Fatal(err)
	}

	return c.doRequest(req, nil)
}

func TestRetryReadsRecoverFromServerErrors(t *testing.T) {
	var calls atomic.Int32
	srv := scripted(t, &calls, 500, 503, 200)

	if _, err := do(t, newRetryTestClient(srv, 4, time.Second), http.MethodGet, ""); err != nil {
		t.Fatalf("GET after two 5xx: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("requests = %d, want 3", got)
	}
}

// TestRetryExhaustedKeepsApplesMessage is what the passthrough error handler
// is for: the provider's diagnostics match on Apple's text, which
// retryablehttp's default handler would have thrown away.
func TestRetryExhaustedKeepsApplesMessage(t *testing.T) {
	var calls atomic.Int32
	srv := scripted(t, &calls, 500)

	_, err := do(t, newRetryTestClient(srv, 2, time.Second), http.MethodGet, "")
	if err == nil {
		t.Fatal("GET succeeded against a server that only answers 500")
	}
	for _, want := range []string{"status 500", "An unexpected error occurred on the server side", "after 3 attempts"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("requests = %d, want 3", got)
	}
}

func TestRetryPostIsNotRepeatedOnServerError(t *testing.T) {
	var calls atomic.Int32
	srv := scripted(t, &calls, 500, 201)

	_, err := do(t, newRetryTestClient(srv, 4, time.Second), http.MethodPost, `{"data":{}}`)
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("POST error = %v, want the 500", err)
	}
	if strings.Contains(err.Error(), "attempts") {
		t.Errorf("a single attempt was reported as several: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("requests = %d, want 1: a 500 on a create may have created the record", got)
	}
}

func TestRetryPostIsRepeatedOnTooManyRequests(t *testing.T) {
	var calls atomic.Int32
	srv := scripted(t, &calls, 429, 201)

	if _, err := do(t, newRetryTestClient(srv, 4, time.Second), http.MethodPost, `{"data":{}}`); err != nil {
		t.Fatalf("POST after a 429: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
}

func TestRetryNeverRepeatsClientErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict} {
		var calls atomic.Int32
		srv := scripted(t, &calls, status)

		if _, err := do(t, newRetryTestClient(srv, 4, time.Second), http.MethodGet, ""); err == nil {
			t.Errorf("GET %d succeeded", status)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("status %d: requests = %d, want 1", status, got)
		}
	}
}

func TestRetryZeroMaxRetriesMakesOneAttempt(t *testing.T) {
	var calls atomic.Int32
	srv := scripted(t, &calls, 500)

	if _, err := do(t, newRetryTestClient(srv, 0, time.Second), http.MethodGet, ""); err == nil {
		t.Fatal("GET succeeded")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("requests = %d, want 1", got)
	}
}

// TestRetryTimeoutIsPerAttempt is the failure the report was about: Apple
// stalls one request past the timeout and answers the next at once. The
// timeout has to bound each attempt; on the wrapper it would bound the lot.
func TestRetryTimeoutIsPerAttempt(t *testing.T) {
	newStallingServer := func(t *testing.T, calls *atomic.Int32) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				select {
				case <-r.Context().Done():
				case <-time.After(time.Second):
				}
				return
			}
			_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"A1"}}`))
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("GET retries", func(t *testing.T) {
		var calls atomic.Int32
		srv := newStallingServer(t, &calls)

		if _, err := do(t, newRetryTestClient(srv, 2, 100*time.Millisecond), http.MethodGet, ""); err != nil {
			t.Fatalf("GET after one stalled attempt: %v", err)
		}
		if got := calls.Load(); got != 2 {
			t.Errorf("requests = %d, want 2", got)
		}
	})

	t.Run("POST does not", func(t *testing.T) {
		var calls atomic.Int32
		srv := newStallingServer(t, &calls)

		_, err := do(t, newRetryTestClient(srv, 2, 100*time.Millisecond), http.MethodPost, `{"data":{}}`)
		if err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
			t.Fatalf("POST error = %v, want the timeout", err)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("requests = %d, want 1", got)
		}
	})
}

func TestRetryResendsTheSameBody(t *testing.T) {
	var bodies []string
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"A1"}}`))
	}))
	t.Cleanup(srv.Close)

	const body = `{"data":{"type":"apps","id":"A1","attributes":{"contentRightsDeclaration":"USES_THIRD_PARTY_CONTENT"}}}`
	if _, err := do(t, newRetryTestClient(srv, 2, time.Second), http.MethodPatch, body); err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	if len(bodies) != 2 || bodies[0] != body || bodies[1] != body {
		t.Errorf("bodies = %q, want the same body twice", bodies)
	}
}

// TestRetryDeleteNotFoundAfterRetry: the first attempt may have deleted the
// record before failing, so the retry's 404 is the deletion, not an error.
// A 404 on the first attempt is still reported.
func TestRetryDeleteNotFoundAfterRetry(t *testing.T) {
	var calls atomic.Int32
	srv := scripted(t, &calls, 503, 404)
	if _, err := do(t, newRetryTestClient(srv, 2, time.Second), http.MethodDelete, ""); err != nil {
		t.Errorf("DELETE 503 then 404: %v, want success", err)
	}

	var first atomic.Int32
	srv = scripted(t, &first, 404)
	if _, err := do(t, newRetryTestClient(srv, 2, time.Second), http.MethodDelete, ""); err == nil {
		t.Error("DELETE answered 404 at once succeeded, want the 404")
	}
	if got := first.Load(); got != 1 {
		t.Errorf("requests = %d, want 1", got)
	}
}

// TestRetryPlainClientMakesOneAttempt keeps every test that builds a Client
// literal around srv.Client() on the behaviour it was written against.
func TestRetryPlainClientMakesOneAttempt(t *testing.T) {
	var calls atomic.Int32
	srv := scripted(t, &calls, 500)
	c := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}

	if _, err := do(t, c, http.MethodGet, ""); err == nil {
		t.Fatal("GET succeeded")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("requests = %d, want 1", got)
	}
}

func TestRetryBackoff(t *testing.T) {
	retryAfter := func(status int, value string) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{value}}}
	}

	if got := retryBackoff(time.Second, 30*time.Second, 0, retryAfter(429, "7")); got != 7*time.Second {
		t.Errorf("Retry-After 7 = %s, want 7s", got)
	}
	if got := retryBackoff(time.Second, 30*time.Second, 0, retryAfter(429, "600")); got != maxRetryAfter {
		t.Errorf("Retry-After 600 = %s, want the %s cap", got, maxRetryAfter)
	}

	for attempt := range 8 {
		got := retryBackoff(time.Second, 30*time.Second, attempt, nil)
		ceiling := min(time.Second<<attempt, 30*time.Second)
		if got < ceiling/2 || got > ceiling {
			t.Errorf("attempt %d: wait %s outside [%s, %s]", attempt, got, ceiling/2, ceiling)
		}
	}
}
