// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// testCertificate mints a self-signed certificate shaped like an Apple-issued
// one, and returns it base64-encoded as Apple returns it plus the PKCS#1 PEM
// that tls_private_key emits.
func testCertificate(t *testing.T) (certificateContent, privateKeyPEM string, key *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         "Apple Development: Terraform (ABCDE12345)",
			OrganizationalUnit: []string{"TEAMID1234"},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating a certificate: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return base64.StdEncoding.EncodeToString(der), string(keyPEM), key
}

func TestNewIdentity(t *testing.T) {
	t.Parallel()

	certificateContent, privateKeyPEM, _ := testCertificate(t)

	identity, err := NewIdentity(certificateContent, privateKeyPEM)
	if err != nil {
		t.Fatalf("NewIdentity() error = %v", err)
	}

	if got, want := identity.CommonName(), "Apple Development: Terraform (ABCDE12345)"; got != want {
		t.Errorf("CommonName() = %q, want %q", got, want)
	}
	if got, want := identity.TeamID(), "TEAMID1234"; got != want {
		t.Errorf("TeamID() = %q, want %q", got, want)
	}
}

// The certificate arrives from Terraform as base64 that may have been wrapped
// across lines, which the shell version handled with `tr -d '[:space:]'`.
func TestNewIdentityAcceptsWrappedBase64(t *testing.T) {
	t.Parallel()

	certificateContent, privateKeyPEM, _ := testCertificate(t)

	var wrapped strings.Builder
	for i := 0; i < len(certificateContent); i += 64 {
		end := min(i+64, len(certificateContent))
		wrapped.WriteString(certificateContent[i:end])
		wrapped.WriteString("\n")
	}

	if _, err := NewIdentity(wrapped.String(), privateKeyPEM); err != nil {
		t.Fatalf("NewIdentity() error = %v", err)
	}
}

// A hand-assembled bundle may carry PEM where Apple would have sent DER.
func TestNewIdentityAcceptsPEMCertificate(t *testing.T) {
	t.Parallel()

	certificateContent, privateKeyPEM, _ := testCertificate(t)

	der, err := base64.StdEncoding.DecodeString(certificateContent)
	if err != nil {
		t.Fatalf("decoding the test certificate: %v", err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	identity, err := NewIdentity(base64.StdEncoding.EncodeToString(certificatePEM), privateKeyPEM)
	if err != nil {
		t.Fatalf("NewIdentity() error = %v", err)
	}
	if identity.CommonName() == "" {
		t.Error("CommonName() is empty")
	}
}

// Importing a mismatched pair succeeds at the keychain level and only fails
// later, at codesign time, with an unhelpful message.
func TestNewIdentityRejectsMismatchedKey(t *testing.T) {
	t.Parallel()

	certificateContent, _, _ := testCertificate(t)
	_, otherKeyPEM, _ := testCertificate(t)

	_, err := NewIdentity(certificateContent, otherKeyPEM)
	if err == nil {
		t.Fatal("NewIdentity() succeeded with a key from a different certificate")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("NewIdentity() error = %v, want a mismatch error", err)
	}
}

func TestNewIdentityErrors(t *testing.T) {
	t.Parallel()

	certificateContent, privateKeyPEM, _ := testCertificate(t)

	tests := map[string]struct {
		certificateContent string
		privateKeyPEM      string
		wantContains       string
	}{
		"certificate is not base64": {
			certificateContent: "not base64 at all!",
			privateKeyPEM:      privateKeyPEM,
			wantContains:       "base64",
		},
		"certificate is not a certificate": {
			certificateContent: base64.StdEncoding.EncodeToString([]byte("hello")),
			privateKeyPEM:      privateKeyPEM,
			wantContains:       "parsing the certificate",
		},
		"key is not PEM": {
			certificateContent: certificateContent,
			privateKeyPEM:      "definitely not a PEM block",
			wantContains:       "does not contain a PEM block",
		},
		"key is encrypted": {
			certificateContent: certificateContent,
			privateKeyPEM: "-----BEGIN RSA PRIVATE KEY-----\n" +
				"Proc-Type: 4,ENCRYPTED\n\nAAAA\n-----END RSA PRIVATE KEY-----\n",
			wantContains: "encrypted",
		},
		"key block type is unknown": {
			certificateContent: certificateContent,
			privateKeyPEM:      "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n",
			wantContains:       "unsupported private key PEM block",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := NewIdentity(test.certificateContent, test.privateKeyPEM)
			if err == nil {
				t.Fatal("NewIdentity() succeeded, want an error")
			}
			if !strings.Contains(err.Error(), test.wantContains) {
				t.Errorf("NewIdentity() error = %v, want it to contain %q", err, test.wantContains)
			}
		})
	}
}

// PKCS#8 and SEC1 are accepted so that a key adopted from elsewhere works
// without conversion, even though tls_private_key emits PKCS#1 for RSA.
func TestDecodePrivateKeyEncodings(t *testing.T) {
	t.Parallel()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating an RSA key: %v", err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatalf("marshalling PKCS#8: %v", err)
	}

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating an EC key: %v", err)
	}
	sec1, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatalf("marshalling SEC1: %v", err)
	}

	tests := map[string]*pem.Block{
		"PKCS#1": {Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)},
		"PKCS#8": {Type: "PRIVATE KEY", Bytes: pkcs8},
		"SEC1":   {Type: "EC PRIVATE KEY", Bytes: sec1},
	}

	for name, block := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := decodePrivateKey(string(pem.EncodeToMemory(block))); err != nil {
				t.Fatalf("decodePrivateKey() error = %v", err)
			}
		})
	}
}

// The container has to round-trip, and it has to do so under the legacy
// algorithms: `security import` rejects the modern AES encoding that OpenSSL 3
// produces by default.
func TestIdentityPKCS12(t *testing.T) {
	t.Parallel()

	certificateContent, privateKeyPEM, key := testCertificate(t)

	identity, err := NewIdentity(certificateContent, privateKeyPEM)
	if err != nil {
		t.Fatalf("NewIdentity() error = %v", err)
	}

	container, err := identity.PKCS12("hunter2")
	if err != nil {
		t.Fatalf("PKCS12() error = %v", err)
	}

	decodedKey, decodedCertificate, err := pkcs12.Decode(container, "hunter2")
	if err != nil {
		t.Fatalf("decoding the container: %v", err)
	}

	decodedRSA, ok := decodedKey.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("decoded key is %T, want *rsa.PrivateKey", decodedKey)
	}
	if !decodedRSA.Equal(key) {
		t.Error("the decoded key is not the one that went in")
	}
	if decodedCertificate.Subject.CommonName != identity.CommonName() {
		t.Errorf("decoded common name = %q, want %q",
			decodedCertificate.Subject.CommonName, identity.CommonName())
	}

	if _, _, err := pkcs12.Decode(container, "wrong"); err == nil {
		t.Error("decoding with the wrong password succeeded")
	}
}

func TestRandomPassword(t *testing.T) {
	t.Parallel()

	first, err := randomPassword()
	if err != nil {
		t.Fatalf("randomPassword() error = %v", err)
	}
	second, err := randomPassword()
	if err != nil {
		t.Fatalf("randomPassword() error = %v", err)
	}

	if len(first) != 48 {
		t.Errorf("randomPassword() length = %d, want 48", len(first))
	}
	if first == second {
		t.Error("randomPassword() returned the same value twice")
	}
}
