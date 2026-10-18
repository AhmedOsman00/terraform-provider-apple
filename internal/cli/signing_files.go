// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// signingDocument is what `signing install` reads: the certificates by role
// and the provisioning profiles by name, as a Terraform output built from
// apple_certificate and apple_profile exports them. Nothing in it is secret.
type signingDocument struct {
	Certificates map[string]signingCertificate `json:"certificates"`
	Profiles     []signingProfile              `json:"-"`
}

type signingCertificate struct {
	CertificateContent string `json:"certificate_content"`
	ExpirationDate     string `json:"expiration_date"`
}

type signingProfile struct {
	Name           string `json:"name"`
	UUID           string `json:"uuid"`
	State          string `json:"state"`
	ExpirationDate string `json:"expiration_date"`
	ProfileContent string `json:"profile_content"`
}

// parseSigningDocument accepts profiles keyed by name or as a list carrying
// their names, the two shapes a Terraform output naturally takes.
func parseSigningDocument(data []byte) (*signingDocument, error) {
	var raw struct {
		Certificates map[string]signingCertificate `json:"certificates"`
		Profiles     json.RawMessage               `json:"profiles"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("the signing document is not JSON: %w", err)
	}
	if raw.Certificates == nil && raw.Profiles == nil {
		return nil, errors.New("that is not a signing document: it has neither certificates nor profiles")
	}

	doc := &signingDocument{Certificates: raw.Certificates}
	if len(raw.Profiles) == 0 || string(raw.Profiles) == "null" {
		return doc, nil
	}

	var byName map[string]signingProfile
	if err := json.Unmarshal(raw.Profiles, &byName); err == nil {
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			profile := byName[name]
			if profile.Name == "" {
				profile.Name = name
			}
			doc.Profiles = append(doc.Profiles, profile)
		}
		return doc, nil
	}

	if err := json.Unmarshal(raw.Profiles, &doc.Profiles); err != nil {
		return nil, errors.New("profiles is neither an object keyed by name nor a list")
	}
	return doc, nil
}

// parseAppleDate reads Apple's timestamps, which carry milliseconds and an
// offset that Go's RFC 3339 parser does not always take.
func parseAppleDate(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if len(value) >= 19 {
		if t, err := time.Parse("2006-01-02T15:04:05", value[:19]); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unreadable date %q", value)
}

// profilePlist reads the Name and UUID out of a .mobileprovision. The file is
// CMS-signed, and the plist sits in it unencrypted, so it is found by its
// delimiters rather than by verifying a signature only Apple's tools need.
func profilePlist(data []byte) (name, uuid string, err error) {
	start := bytes.Index(data, []byte("<?xml"))
	end := bytes.Index(data, []byte("</plist>"))
	if start < 0 || end < start {
		return "", "", errors.New("not a provisioning profile: no property list inside")
	}

	decoder := xml.NewDecoder(bytes.NewReader(data[start : end+len("</plist>")]))
	depth, key, inKey, inString := 0, "", false, false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", "", fmt.Errorf("not a provisioning profile: %w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "dict":
				depth++
			case "key":
				inKey = depth == 1
			case "string":
				inString = depth == 1
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "dict":
				depth--
			case "key":
				inKey = false
			case "string":
				inString = false
				key = ""
			default:
				if depth == 1 {
					key = ""
				}
			}
		case xml.CharData:
			switch {
			case inKey:
				key = string(t)
			case inString && key == "Name":
				name = string(t)
			case inString && key == "UUID":
				uuid = string(t)
			}
		}
	}
	if name == "" || uuid == "" {
		return "", "", errors.New("not a provisioning profile: no Name or UUID")
	}
	return name, uuid, nil
}

// parsePrivateKey reads an RSA key in PKCS#1 or PKCS#8 PEM, the two forms
// `openssl genrsa` writes depending on its version.
func parsePrivateKey(data []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("not a PEM private key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("not a private key: %w", err)
	}
	signer, ok := parsed.(crypto.Signer)
	if !ok {
		return nil, errors.New("not a signing key")
	}
	return signer, nil
}

// keyMatches reports whether the key is the half of the certificate that signs.
func keyMatches(certificate *x509.Certificate, key crypto.Signer) bool {
	public, ok := key.Public().(interface{ Equal(crypto.PublicKey) bool })
	return ok && public.Equal(certificate.PublicKey)
}

// The encrypted keys are `openssl enc -aes-256-cbc -pbkdf2 -iter 600000 -a`:
// base64 of "Salted__", eight bytes of salt and the ciphertext, the key and IV
// derived together with PBKDF2-HMAC-SHA256. Keeping the format means files
// encrypted by hand, or by an older script, still open.
const (
	encIterations = 600000
	encMagic      = "Salted__"
)

func deriveKeyIV(passphrase string, salt []byte) (key, iv []byte, err error) {
	derived, err := pbkdf2.Key(sha256.New, passphrase, salt, encIterations, 32+aes.BlockSize)
	if err != nil {
		return nil, nil, err
	}
	return derived[:32], derived[32:], nil
}

func decryptKey(data []byte, passphrase string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(string(data)), ""))
	if err != nil {
		return nil, errors.New("not an encrypted key: the file is not base64")
	}
	if len(raw) < 16+aes.BlockSize || string(raw[:8]) != encMagic || (len(raw)-16)%aes.BlockSize != 0 {
		return nil, errors.New("not an encrypted key: no salt header")
	}

	key, iv, err := deriveKeyIV(passphrase, raw[8:16])
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(raw)-16)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, raw[16:])

	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > aes.BlockSize || !bytes.Equal(plain[len(plain)-pad:], bytes.Repeat([]byte{byte(pad)}, pad)) {
		return nil, errors.New("wrong passphrase")
	}
	plain = plain[:len(plain)-pad]
	if _, err := parsePrivateKey(plain); err != nil {
		return nil, errors.New("wrong passphrase")
	}
	return plain, nil
}

func encryptKey(plain []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, iv, err := deriveKeyIV(passphrase, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	pad := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(append([]byte{}, plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	sealed := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(sealed, padded)

	encoded := base64.StdEncoding.EncodeToString(append(append([]byte(encMagic), salt...), sealed...))
	var out strings.Builder
	for len(encoded) > 64 {
		out.WriteString(encoded[:64] + "\n")
		encoded = encoded[64:]
	}
	out.WriteString(encoded + "\n")
	return []byte(out.String()), nil
}
