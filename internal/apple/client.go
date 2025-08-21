package apple

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	"terraform-provider-apple/internal/apple/models"

	"github.com/golang-jwt/jwt/v5"
)

const HostURL string = "https://api.appstoreconnect.apple.com"

type Client struct {
	HostURL    string
	HTTPClient *http.Client
	Token      string
	IssuerID   string
	KeyID      string
	PrivateKey string
	Scope      []string
}

// NewClient creates a new Apple API client with a generated JWT token.
func NewClient(issuerID, keyID, privateKeyPEM string, scope []string) (*Client, error) {
	token, err := createToken(keyID, issuerID, privateKeyPEM, scope)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %v", err)
	}

	return &Client{
		HostURL:    HostURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Token:      token,
		IssuerID:   issuerID,
		KeyID:      keyID,
		PrivateKey: privateKeyPEM,
		Scope:      scope,
	}, nil
}

// RefreshToken generates a new JWT token when the current one expires
func (c *Client) RefreshToken() error {
	token, err := createToken(c.KeyID, c.IssuerID, c.PrivateKey, c.Scope)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %v", err)
	}
	c.Token = token
	return nil
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
		"iss":   issuerID,
		"iat":   issuedAt,
		"exp":   expiresAt,
		"aud":   audience,
		"scope": scope,
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

	res, err := c.HTTPClient.Do(req)
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

	// Handle error responses
	if res.StatusCode == 401 {
		// Token might be expired, try to refresh
		if err := c.RefreshToken(); err != nil {
			return nil, fmt.Errorf("authentication failed and token refresh failed: %v", err)
		}
		// Retry the request with new token
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
		res, err = c.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		body, err = io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return body, nil
		}
	}

	// Parse error response
	var errorResponse models.ErrorResponse
	if err := json.Unmarshal(body, &errorResponse); err == nil && len(errorResponse.Errors) > 0 {
		return nil, fmt.Errorf("API error (status %d): %s - %s",
			res.StatusCode, errorResponse.Errors[0].Title, errorResponse.Errors[0].Detail)
	}

	return nil, fmt.Errorf("HTTP error: status %d, body: %s", res.StatusCode, body)
}
