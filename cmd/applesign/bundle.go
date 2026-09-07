package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Bundle is the signing material produced by the Terraform module in
// examples/signing. It is deliberately the only thing this command understands:
// where the JSON came from -- Terraform, sops, a password manager -- is the
// caller's business, so any source composes as a pipe.
type Bundle struct {
	BundleIdentifier string        `json:"bundle_identifier"`
	Certificates     []Certificate `json:"certificates"`
	Profiles         []Profile     `json:"profiles"`
}

// Certificate is one signing identity: the Apple-issued certificate plus the
// private key that was generated locally and never sent to Apple.
type Certificate struct {
	Key                string `json:"key"`
	CertificateType    string `json:"certificate_type"`
	CertificateContent string `json:"certificate_content"`
	PrivateKeyPEM      string `json:"private_key_pem"`
	ExpirationDate     string `json:"expiration_date"`
}

// Profile is one provisioning profile, with its content base64-encoded exactly
// as Apple returns it.
type Profile struct {
	Name           string `json:"name"`
	UUID           string `json:"uuid"`
	ProfileType    string `json:"profile_type"`
	ProfileContent string `json:"profile_content"`
	ExpirationDate string `json:"expiration_date"`
}

// DecodeBundle reads a signing bundle as JSON and checks that it holds enough
// to install something. Unknown fields are tolerated so that a newer module can
// add to the bundle without breaking an older applesign.
func DecodeBundle(r io.Reader) (*Bundle, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading the signing bundle: %w", err)
	}

	trimmed := strings.TrimSpace(string(raw))
	switch trimmed {
	case "":
		return nil, errors.New("the signing bundle is empty; has the module been applied?")
	case "null":
		return nil, errors.New("the signing bundle is null; has the module been applied?")
	}

	var bundle Bundle
	if err := json.Unmarshal([]byte(trimmed), &bundle); err != nil {
		return nil, fmt.Errorf("the signing bundle is not valid JSON: %w", err)
	}

	if err := bundle.validate(); err != nil {
		return nil, err
	}

	return &bundle, nil
}

// readBundle opens the install source: "-" is stdin, anything else is a path.
// Paths are accepted mainly so that process substitution works --
// `applesign install <(sops -d bundle.enc.json)` hands over /dev/fd/63.
func readBundle(source string) (*Bundle, error) {
	if source == "-" {
		return DecodeBundle(os.Stdin)
	}

	file, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("opening the signing bundle: %w", err)
	}
	defer file.Close() //nolint:errcheck // read-only.

	return DecodeBundle(file)
}

// validate rejects a bundle that would fail halfway through installing, so that
// a typo in the source is reported before anything touches the keychain.
func (b *Bundle) validate() error {
	if len(b.Certificates) == 0 {
		return errors.New("no certificates in the signing bundle")
	}

	for i, certificate := range b.Certificates {
		label := certificate.Key
		if label == "" {
			label = fmt.Sprintf("certificate %d", i)
		}
		if certificate.CertificateContent == "" {
			return fmt.Errorf("%s: certificate_content is empty", label)
		}
		if certificate.PrivateKeyPEM == "" {
			return fmt.Errorf("%s: private_key_pem is empty; the bundle carries no usable identity", label)
		}
	}

	return nil
}

// label names a certificate in output, falling back to its type when the module
// did not record a key.
func (c Certificate) label() string {
	if c.Key != "" {
		return c.Key
	}
	if c.CertificateType != "" {
		return c.CertificateType
	}
	return "certificate"
}
