// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-retryablehttp"
)

// App Store Connect answers an occasional request with a 500, or stalls it
// past the client timeout, while the same request a second later answers in
// a second. A refresh reads every resource in parallel, so with a few dozen
// requests per plan one of them failing is the normal case rather than the
// exception, and each such failure fails the whole run. The client therefore
// retries what is safe to retry.
//
// What is safe depends on the method, which is why there are two policies and
// two clients rather than one: retryablehttp's CheckRetry sees the response
// and the error, not the request, and a timed-out request has no response.
//
//   - GET, PATCH and DELETE retry on 429, on 500/502/503/504 and on a timeout
//     or dropped connection. Every PATCH this client sends is a full set of
//     attributes or a whole relationship, so repeating one is harmless.
//   - POST retries on 429 alone. A 429 is refused before Apple acts on it; a
//     500 or a timeout may land after Apple created the record, and repeating
//     the create would duplicate it or send a second invitation. Where a
//     create is known to fail transiently, the caller looks before retrying
//     itself -- see CreateProfile.
//   - 401 and every other 4xx are never retried: they are answers, not
//     accidents.
const (
	DefaultMaxRetries     = 4
	DefaultRequestTimeout = 30 * time.Second

	retryWaitMin = 1 * time.Second
	retryWaitMax = 30 * time.Second
	// Retry-After is honoured up to this long; past it, the wait would look
	// like a hang.
	maxRetryAfter = 60 * time.Second
)

// retryPolicy is one of readRetryPolicy or writeRetryPolicy.
type retryPolicy = retryablehttp.CheckRetry

// newAPIHTTPClient builds an App Store Connect client that retries under the
// given policy. timeout applies to each attempt, not to the call as a whole:
// it is set on the inner client, never on the wrapper StandardClient returns.
func newAPIHTTPClient(timeout time.Duration, maxRetries int, policy retryPolicy) *http.Client {
	return newRetryingClient(timeout, maxRetries, policy, retryWaitMin, retryWaitMax)
}

func newRetryingClient(timeout time.Duration, maxRetries int, policy retryPolicy, waitMin, waitMax time.Duration) *http.Client {
	inner := cleanhttp.DefaultPooledClient()
	inner.Timeout = timeout

	rc := &retryablehttp.Client{
		HTTPClient:   inner,
		Logger:       nil, // the default writes to stderr
		RetryWaitMin: waitMin,
		RetryWaitMax: waitMax,
		RetryMax:     maxRetries,
		CheckRetry:   policy,
		Backoff:      retryBackoff,
		// Without this, exhausting the retries discards Apple's response and
		// returns "giving up after N attempts", which would hide the error
		// text every diagnostic in the provider matches on.
		ErrorHandler:   retryablehttp.PassthroughErrorHandler,
		RequestLogHook: countAttempt,
	}

	return rc.StandardClient()
}

// readRetryPolicy is the policy for GET, PATCH and DELETE.
func readRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if err != nil {
		return isTransientNetErr(err), nil
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true, nil
	}

	return false, nil
}

// writeRetryPolicy is the policy for POST: only a request Apple refused
// before acting on it is repeated.
func writeRetryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// The transport error is not lost: the passthrough handler returns it.
	if err != nil {
		return false, nil //nolint:nilerr // reported by PassthroughErrorHandler, not by the policy
	}

	return resp.StatusCode == http.StatusTooManyRequests, nil
}

// isTransientNetErr reports whether a transport error is worth another
// attempt: a timeout or a connection that dropped or never opened. A
// certificate Go refuses will be refused again.
func isTransientNetErr(err error) bool {
	var certErr *tls.CertificateVerificationError
	var authorityErr x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	if errors.As(err, &certErr) || errors.As(err, &authorityErr) || errors.As(err, &hostnameErr) {
		return false
	}

	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// *url.Error implements net.Error whatever it wraps, so ask it directly
	// rather than letting errors.As match it as a net.Error below.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Timeout()
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// retryBackoff is retryablehttp's exponential backoff with equal jitter, so
// requests that failed together do not all come back together, and with
// Retry-After honoured up to maxRetryAfter.
func retryBackoff(waitMin, waitMax time.Duration, attempt int, resp *http.Response) time.Duration {
	if resp != nil && resp.Header.Get("Retry-After") != "" &&
		(resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) {
		// DefaultBackoff returns the header's value when it parses, and the
		// exponential wait when it does not.
		return min(retryablehttp.DefaultBackoff(waitMin, waitMax, attempt, resp), maxRetryAfter)
	}

	wait := retryablehttp.DefaultBackoff(waitMin, waitMax, attempt, nil)
	half := wait / 2
	if half <= 0 {
		return wait
	}

	return half + rand.N(half+1)
}

// attemptCounterKey carries a *int through the request context, which
// countAttempt sets to the number of attempts made so far. doRequest reads it
// to word a final error and to recognise a DELETE whose earlier attempt may
// already have succeeded.
type attemptCounterKey struct{}

func withAttemptCounter(ctx context.Context, counter *int) context.Context {
	return context.WithValue(ctx, attemptCounterKey{}, counter)
}

// countAttempt is the RequestLogHook; attempt is zero-based.
func countAttempt(_ retryablehttp.Logger, req *http.Request, attempt int) {
	if counter, ok := req.Context().Value(attemptCounterKey{}).(*int); ok {
		*counter = attempt + 1
	}
}
