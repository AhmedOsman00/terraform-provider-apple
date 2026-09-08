// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// testKeyPEM generates a throwaway P-256 key in the PKCS#8 PEM form App Store
// Connect issues, so the token tests need no credentials.
func testKeyPEM(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling key: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func parseClaims(t *testing.T, token string) jwt.MapClaims {
	t.Helper()

	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		t.Fatalf("parsing token: %v", err)
	}

	return claims
}

// TestCreateTokenOmitsEmptyScope guards the bug that broke every API call: a
// token carrying "scope": [] or "scope": null is rejected by App Store Connect
// with a 400 ENTITY_INVALID titled "JSON processing failed". The provider
// defaults to no scope, so the claim has to be absent rather than empty.
func TestCreateTokenOmitsEmptyScope(t *testing.T) {
	keyPEM := testKeyPEM(t)

	for name, scope := range map[string][]string{
		"nil scope":   nil,
		"empty scope": {},
	} {
		t.Run(name, func(t *testing.T) {
			token, err := createToken("KEYID12345", "issuer-id", keyPEM, scope)
			if err != nil {
				t.Fatalf("createToken: %v", err)
			}

			if _, ok := parseClaims(t, token)["scope"]; ok {
				t.Error("scope claim present; App Store Connect rejects an empty scope with a 400")
			}
		})
	}
}

// TestCreateTokenKeepsPopulatedScope confirms the omission is conditional: a
// caller that genuinely restricts the token still gets the claim.
func TestCreateTokenKeepsPopulatedScope(t *testing.T) {
	scope := []string{"GET /v1/apps"}

	token, err := createToken("KEYID12345", "issuer-id", testKeyPEM(t), scope)
	if err != nil {
		t.Fatalf("createToken: %v", err)
	}

	claims := parseClaims(t, token)

	got, ok := claims["scope"].([]interface{})
	if !ok {
		t.Fatalf("scope claim missing or not a list: %#v", claims["scope"])
	}

	if len(got) != 1 || got[0] != scope[0] {
		t.Errorf("scope = %#v, want %#v", got, scope)
	}
}

// TestCreateTokenHeaderAndClaims covers the rest of the token contract: Apple
// matches the key by the "kid" header and the team by the issuer claim.
func TestCreateTokenHeaderAndClaims(t *testing.T) {
	token, err := createToken("KEYID12345", "issuer-id", testKeyPEM(t), nil)
	if err != nil {
		t.Fatalf("createToken: %v", err)
	}

	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("parsing token: %v", err)
	}

	if kid := parsed.Header["kid"]; kid != "KEYID12345" {
		t.Errorf("kid header = %v, want KEYID12345", kid)
	}

	if alg := parsed.Header["alg"]; alg != "ES256" {
		t.Errorf("alg header = %v, want ES256", alg)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims type %T", parsed.Claims)
	}

	if claims["iss"] != "issuer-id" {
		t.Errorf("iss = %v, want issuer-id", claims["iss"])
	}

	if claims["aud"] != "appstoreconnect-v1" {
		t.Errorf("aud = %v, want appstoreconnect-v1", claims["aud"])
	}
}
