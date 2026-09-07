// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"unicode"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// Identity is a certificate paired with the private key that signs for it,
// ready to be written as a PKCS#12 file for `security import`.
type Identity struct {
	Certificate *x509.Certificate
	PrivateKey  crypto.PrivateKey
}

// NewIdentity decodes one bundle certificate and verifies that the key belongs
// to it. Importing a mismatched pair succeeds at the keychain level and then
// fails much later with "no identity found", so the check is worth its cost.
func NewIdentity(certificateContent, privateKeyPEM string) (*Identity, error) {
	certificate, err := decodeCertificate(certificateContent)
	if err != nil {
		return nil, err
	}

	privateKey, err := decodePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}

	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("private key of type %T cannot sign", privateKey)
	}

	certificateKey, ok := certificate.PublicKey.(interface{ Equal(crypto.PublicKey) bool })
	if !ok {
		return nil, fmt.Errorf("certificate public key of type %T cannot be compared", certificate.PublicKey)
	}

	if !certificateKey.Equal(signer.Public()) {
		return nil, errors.New("the private key does not match the certificate")
	}

	return &Identity{Certificate: certificate, PrivateKey: privateKey}, nil
}

// CommonName is the name codesign matches an identity by. Apple issues the
// certificate under the team's own name, so this is not the CSR subject.
func (i *Identity) CommonName() string {
	return i.Certificate.Subject.CommonName
}

// TeamID is Apple's team identifier, which it places in the subject's
// organizational unit.
func (i *Identity) TeamID() string {
	if len(i.Certificate.Subject.OrganizationalUnit) == 0 {
		return ""
	}
	return i.Certificate.Subject.OrganizationalUnit[0]
}

// PKCS12 encodes the identity for `security import`.
//
// The encoding is deliberately the legacy one: certificates under PBE-SHA1-RC2
// and the key under PBE-SHA1-3DES, which is what OpenSSL produced before 3.0
// and what the macOS keychain reads. The modern AES encoding, which is what
// OpenSSL 3 emits by default, is rejected by `security import`.
func (i *Identity) PKCS12(password string) ([]byte, error) {
	data, err := pkcs12.LegacyRC2.Encode(i.PrivateKey, i.Certificate, nil, password)
	if err != nil {
		return nil, fmt.Errorf("assembling the PKCS#12 container: %w", err)
	}
	return data, nil
}

// decodeCertificate accepts Apple's base64-encoded DER, with or without the
// line wrapping Terraform may carry through, and tolerates a PEM payload in
// case a bundle is assembled by hand.
func decodeCertificate(content string) (*x509.Certificate, error) {
	der, err := base64.StdEncoding.DecodeString(stripWhitespace(content))
	if err != nil {
		return nil, fmt.Errorf("decoding certificate_content as base64: %w", err)
	}

	if block, _ := pem.Decode(der); block != nil {
		der = block.Bytes
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parsing the certificate: %w", err)
	}

	return certificate, nil
}

// decodePrivateKey reads the PEM that tls_private_key emits. RSA keys arrive as
// PKCS#1 and ECDSA keys as SEC1, but PKCS#8 is accepted too so that a key
// adopted from elsewhere works without conversion.
func decodePrivateKey(keyPEM string) (crypto.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, errors.New("private_key_pem does not contain a PEM block")
	}

	// Both spellings of an encrypted PEM: PKCS#8's block type, and the
	// Proc-Type header OpenSSL's traditional format uses.
	if block.Type == "ENCRYPTED PRIVATE KEY" || strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
		return nil, errors.New("the private key is encrypted; applesign has no way to ask for a passphrase")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing the PKCS#1 private key: %w", err)
		}
		return key, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing the SEC1 private key: %w", err)
		}
		return key, nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing the PKCS#8 private key: %w", err)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported private key PEM block %q", block.Type)
	}
}

// decodeContent decodes a base64 payload that may have been wrapped across
// lines, which is how profile content comes back.
func decodeContent(content string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(stripWhitespace(content))
}

// stripWhitespace removes the line wrapping base64 payloads may carry.
func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// randomPassword returns a throwaway password for a PKCS#12 container or a CI
// keychain. Both live only as long as the process that created them.
func randomPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating a random password: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
