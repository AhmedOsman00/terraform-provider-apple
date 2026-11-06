// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/golang-jwt/jwt/v5"
)

const HostURL string = "https://api.appstoreconnect.apple.com"

type Client struct {
	HostURL string
	// HTTPClient carries GET, PATCH and DELETE, and POST too when
	// WriteHTTPClient is nil. NewClient builds it to retry transient failures
	// (see retry.go); a plain *http.Client, as tests use, makes one attempt.
	HTTPClient *http.Client
	// WriteHTTPClient carries POST and anything else asCreate marks, under the
	// narrower policy a create needs: a 500 or a timeout may arrive after Apple
	// created the record.
	WriteHTTPClient *http.Client
	// UploadHTTPClient delivers media file parts to the presigned URLs Apple
	// hands out for them. It is deliberately not HTTPClient -- see uploads.go.
	// Left nil, a shared default with its own transport is used.
	UploadHTTPClient *http.Client
	Token            string
	IssuerID         string
	KeyID            string
}

// ClientOptions configures NewClient. The zero value is the default.
type ClientOptions struct {
	// Scope restricts the token to named operations; see createToken.
	Scope []string
	// MaxRetries is how many times a transient failure is retried. Nil means
	// DefaultMaxRetries; zero turns retries off.
	MaxRetries *int
	// RequestTimeout bounds each attempt. Zero means DefaultRequestTimeout.
	RequestTimeout time.Duration
}

// NewClient creates a new Apple API client with a generated JWT token.
//
// The token is minted once here and is immutable for the life of the client.
// App Store Connect tokens are valid for 20 minutes, which comfortably outlives
// a single Terraform command: every operation this provider performs is one
// fast REST call, and `plan` and `apply` run as separate processes that each
// construct their own client. The signing key is therefore not retained.
func NewClient(issuerID, keyID, privateKeyPEM string, opts ClientOptions) (*Client, error) {
	token, err := createToken(keyID, issuerID, privateKeyPEM, opts.Scope)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %v", err)
	}

	maxRetries := DefaultMaxRetries
	if opts.MaxRetries != nil {
		maxRetries = *opts.MaxRetries
	}

	timeout := opts.RequestTimeout
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}

	return &Client{
		HostURL:          HostURL,
		HTTPClient:       newAPIHTTPClient(timeout, maxRetries, readRetryPolicy),
		WriteHTTPClient:  newAPIHTTPClient(timeout, maxRetries, writeRetryPolicy),
		UploadHTTPClient: newUploadHTTPClient(),
		Token:            token,
		IssuerID:         issuerID,
		KeyID:            keyID,
	}, nil
}

func createToken(keyID, issuerID, privateKeyPEM string, scope []string) (string, error) {
	audience := "appstoreconnect-v1"

	// Token expiration (20 min from now)
	now := time.Now()
	issuedAt := now.Unix()
	expiresAt := now.Add(20 * time.Minute).Unix()

	// Decode the private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil || block.Type != "PRIVATE KEY" {
		return "", fmt.Errorf("failed to decode PEM block containing the private key")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %v", err)
	}

	// Create the JWT claims
	claims := jwt.MapClaims{
		"iss": issuerID,
		"iat": issuedAt,
		"exp": expiresAt,
		"aud": audience,
	}

	// The scope claim is omitted unless it actually restricts something. App
	// Store Connect rejects a token carrying an empty or null scope with a 400
	// ENTITY_INVALID titled "JSON processing failed" — an error about the token
	// payload that reads like a complaint about the request body. Since the
	// provider defaults to no scope, sending the claim unconditionally broke
	// every API call.
	if len(scope) > 0 {
		claims["scope"] = scope
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)

	// Set the "kid" in the header
	token.Header["kid"] = keyID

	// Sign the token with the private key
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	return signedToken, nil
}

func (c *Client) doRequest(req *http.Request, authToken *string) ([]byte, error) {
	token := c.Token

	if authToken != nil {
		token = *authToken
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	attempts := 0
	req = req.WithContext(withAttemptCounter(req.Context(), &attempts))

	body, err := c.send(req, &attempts)
	if err != nil && attempts > 1 {
		return nil, fmt.Errorf("%w (after %d attempts)", err, attempts)
	}

	return body, err
}

// send is one doRequest after the headers are set. attempts is filled in by
// the retrying client as it goes, and stays zero under a plain one.
func (c *Client) send(req *http.Request, attempts *int) ([]byte, error) {
	client := c.HTTPClient
	if createsRecords(req) && c.WriteHTTPClient != nil {
		client = c.WriteHTTPClient
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// Handle different success status codes
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return body, nil
	}

	// A DELETE that timed out or met a 5xx may have gone through before the
	// failure; the retry then finds nothing to delete. Reporting that 404
	// would fail a destroy that worked.
	if req.Method == http.MethodDelete && res.StatusCode == http.StatusNotFound && *attempts > 1 {
		return nil, nil
	}

	// Prefer Apple's own structured error message when it supplies one.
	var errorResponse models.ErrorResponse
	// Apple lists one error per problem -- a refused review submission names
	// every missing piece -- so all of them are reported, not only the first.
	if err := json.Unmarshal(body, &errorResponse); err == nil && len(errorResponse.Errors) > 0 {
		messages := make([]string, 0, len(errorResponse.Errors))
		for _, apiErr := range errorResponse.Errors {
			messages = append(messages, fmt.Sprintf("%s - %s", apiErr.Title, apiErr.Detail))
		}

		return nil, fmt.Errorf("API error (status %d): %s", res.StatusCode, strings.Join(messages, "; "))
	}

	// A 401 means the credentials themselves were rejected. Re-signing a token
	// with the same key would produce an equivalent token and be rejected
	// identically, so report the failure instead of retrying.
	if res.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed (status 401): App Store Connect rejected the API credentials. "+
			"Verify the issuer ID, key ID, and private key are correct and that the key has not been revoked in "+
			"App Store Connect under Users and Access > Keys. Response: %s", body)
	}

	return nil, fmt.Errorf("HTTP error: status %d, body: %s", res.StatusCode, body)
}
