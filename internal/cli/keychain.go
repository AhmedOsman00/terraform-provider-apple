// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"crypto"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

const wwdrURL = "https://www.apple.com/certificateauthority/AppleWWDRCAG3.cer"

// keychain drives /usr/bin/security against one keychain file.
type keychain struct {
	path string
}

func loginKeychain() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Keychains", "login.keychain-db")
}

// ephemeralKeychainPath is where a throwaway keychain goes when --keychain
// does not say: the runner's temporary directory, which the job discards.
func ephemeralKeychainPath() string {
	dir := os.Getenv("RUNNER_TEMP")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "apple-signing.keychain-db")
}

func security(args ...string) (string, error) {
	out, err := exec.Command("/usr/bin/security", args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("security %s: %v: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// create makes a keychain nothing else uses, with a random password, unlocked
// for six hours and first in the search list so codesign finds it. A keychain
// left at the path by an earlier run is replaced.
func (k keychain) create() (password string, err error) {
	if _, err := os.Stat(k.path); err == nil {
		if _, err := security("delete-keychain", k.path); err != nil {
			return "", err
		}
	}

	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	password = hex.EncodeToString(secret)

	if _, err := security("create-keychain", "-p", password, k.path); err != nil {
		return "", err
	}
	if _, err := security("set-keychain-settings", "-lut", "21600", k.path); err != nil {
		return "", err
	}
	if _, err := security("unlock-keychain", "-p", password, k.path); err != nil {
		return "", err
	}

	list, err := security("list-keychains", "-d", "user")
	if err != nil {
		return "", err
	}
	args := []string{"list-keychains", "-d", "user", "-s", k.path}
	for _, line := range strings.Split(list, "\n") {
		if entry := strings.Trim(strings.TrimSpace(line), `"`); entry != "" && entry != k.path {
			args = append(args, entry)
		}
	}
	if _, err := security(args...); err != nil {
		return "", err
	}

	return password, k.importWWDR()
}

// importWWDR adds Apple's intermediate, without which an issued certificate
// does not chain to a trusted root and codesign reports no identity at all.
func (k keychain) importWWDR() error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(wwdrURL)
	if err != nil {
		return fmt.Errorf("downloading Apple's WWDR intermediate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading Apple's WWDR intermediate: %s", resp.Status)
	}

	file, err := os.CreateTemp("", "wwdr-*.cer")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		return err
	}
	file.Close()

	_, err = security("import", file.Name(), "-k", k.path)
	return err
}

func (k keychain) hasWWDR() bool {
	out, err := security("find-certificate", "-c", "Apple Worldwide Developer Relations Certification Authority", "-a", "-Z")
	return err == nil && strings.Contains(out, "SHA-1 hash")
}

func (k keychain) hasCertificate(certificate *x509.Certificate) bool {
	sum := sha1.Sum(certificate.Raw) //nolint:gosec
	out, err := security("find-certificate", "-a", "-Z", k.path)
	return err == nil && strings.Contains(strings.ToUpper(out), "SHA-1 HASH: "+strings.ToUpper(hex.EncodeToString(sum[:])))
}

// importIdentity pairs the certificate with its key in a PKCS#12 container and
// imports it. The container is the legacy kind: `security import` refuses the
// AES one modern OpenSSL writes by default. -T grants the key to the signing
// tools only, not to every process.
func (k keychain) importIdentity(certificate *x509.Certificate, key crypto.Signer) error {
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	password := hex.EncodeToString(secret)

	data, err := pkcs12.Legacy.Encode(key, certificate, nil, password)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "apple-signing-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "identity.p12")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	_, err = security("import", path, "-k", k.path, "-P", password, "-f", "pkcs12",
		"-T", "/usr/bin/codesign", "-T", "/usr/bin/security", "-T", "/usr/bin/productbuild")
	return err
}

// allowCodesign lets codesign use the keys without a prompt nobody on a runner
// could answer. It needs the keychain password, so it works only on a keychain
// this tool created.
func (k keychain) allowCodesign(password string) error {
	_, err := security("set-key-partition-list", "-S", "apple-tool:,apple:,codesign:", "-s", "-k", password, k.path)
	return err
}

func (k keychain) identities() string {
	out, _ := security("find-identity", "-v", "-p", "codesigning", k.path)
	return strings.TrimSpace(out)
}

func (k keychain) remove() (bool, error) {
	if _, err := os.Stat(k.path); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	_, err := security("delete-keychain", k.path)
	return err == nil, err
}
