// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// TestDecryptKeyOpensWhatOpenSSLWrote uses a file written by
// `openssl enc -aes-256-cbc -pbkdf2 -iter 600000 -a`, the format keys
// encrypted before this tool existed are in.
func TestDecryptKeyOpensWhatOpenSSLWrote(t *testing.T) {
	t.Parallel()

	sealed, err := os.ReadFile("testdata/fixture.key.enc")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/fixture.key")
	if err != nil {
		t.Fatal(err)
	}

	got, err := decryptKey(sealed, "correct horse")
	if err != nil {
		t.Fatalf("decryptKey: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("decryptKey did not return the original key")
	}

	if _, err := decryptKey(sealed, "wrong"); err == nil {
		t.Fatal("decryptKey accepted the wrong passphrase")
	}
}

// TestEncryptKeyIsReadableByOpenSSL checks the other direction, so a file this
// tool writes can still be opened by hand.
func TestEncryptKeyIsReadableByOpenSSL(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat("/usr/bin/openssl"); err != nil {
		t.Skip("no /usr/bin/openssl")
	}
	plain, err := os.ReadFile("testdata/fixture.key")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := encryptKey(plain, "battery staple")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "role.key.enc")
	if err := os.WriteFile(path, sealed, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("/usr/bin/openssl", "enc", "-d", "-aes-256-cbc", "-pbkdf2", "-iter", "600000", "-a",
		"-in", path, "-pass", "pass:battery staple")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("openssl could not decrypt: %v", err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatal("openssl decrypted something else")
	}
}

func TestParseSigningDocumentTakesBothProfileShapes(t *testing.T) {
	t.Parallel()

	byName, err := parseSigningDocument([]byte(`{
		"certificates": {"distribution": {"certificate_content": "AA==", "expiration_date": "2027-01-01T00:00:00.000+00:00"}},
		"profiles": {"App Store": {"uuid": "U1", "state": "ACTIVE", "profile_content": "AA=="}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(byName.Profiles) != 1 || byName.Profiles[0].Name != "App Store" || byName.Profiles[0].UUID != "U1" {
		t.Fatalf("profiles keyed by name: %+v", byName.Profiles)
	}

	list, err := parseSigningDocument([]byte(`{"profiles": [{"name": "Dev", "uuid": "U2", "profile_content": "AA=="}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Profiles) != 1 || list.Profiles[0].Name != "Dev" {
		t.Fatalf("profiles as a list: %+v", list.Profiles)
	}

	if _, err := parseSigningDocument([]byte(`{"bundle_ids": {}}`)); err == nil {
		t.Fatal("accepted a document with neither certificates nor profiles")
	}
}

func TestParseAppleDate(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"2027-03-01T08:00:00Z", "2027-03-01T08:00:00.000+00:00"} {
		got, err := parseAppleDate(value)
		if err != nil || got.Year() != 2027 || got.Hour() != 8 {
			t.Errorf("parseAppleDate(%s) = %v, %v", value, got, err)
		}
	}
}

func TestProfilePlistReadsTheTopLevelNameAndUUID(t *testing.T) {
	t.Parallel()

	profile := []byte("\x30\x82signed-data<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		`<plist version="1.0"><dict>
		<key>AppIDName</key><string>App</string>
		<key>Entitlements</key><dict><key>Name</key><string>nested</string></dict>
		<key>Name</key><string>App Store</string>
		<key>ProvisionedDevices</key><array><string>UDID</string></array>
		<key>UUID</key><string>1234-ABCD</string>
		</dict></plist>` + "\x00signature")

	name, uuid, err := profilePlist(profile)
	if err != nil {
		t.Fatal(err)
	}
	if name != "App Store" || uuid != "1234-ABCD" {
		t.Fatalf("profilePlist = %q %q; want App Store 1234-ABCD", name, uuid)
	}

	if _, _, err := profilePlist([]byte("not a profile")); err == nil {
		t.Fatal("profilePlist accepted a file with no plist")
	}
}

// TestIdentityPairsAndEncodesAsLegacyPKCS12 covers the two checks before an
// import: the key is the certificate's, and the container round-trips.
func TestIdentityPairsAndEncodesAsLegacyPKCS12(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/fixture.key")
	if err != nil {
		t.Fatal(err)
	}
	key, err := parsePrivateKey(data)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Apple Distribution: Test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	if !keyMatches(certificate, key) {
		t.Fatal("keyMatches refused the certificate's own key")
	}
	other, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if keyMatches(certificate, other) {
		t.Fatal("keyMatches accepted another key")
	}

	encoded, err := pkcs12.Legacy.Encode(key, certificate, nil, "secret")
	if err != nil {
		t.Fatal(err)
	}
	_, decoded, err := pkcs12.Decode(encoded, "secret")
	if err != nil || decoded.Subject.CommonName != "Apple Distribution: Test" {
		t.Fatalf("the container did not round-trip: %v", err)
	}
}
