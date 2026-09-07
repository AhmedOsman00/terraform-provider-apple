package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// securityPath is the keychain tool. Everything below shells out to it because
// there is no cgo-free Go binding for the Security framework.
const securityPath = "/usr/bin/security"

// defaultKeychainName is where --ci installs unless --keychain says otherwise.
const defaultKeychainName = "apple-signing.keychain-db"

// security runs the keychain tool, discarding its output. Errors carry the tool
// output, which is where the useful part of a keychain failure lives.
func security(args ...string) error {
	_, err := securityOutput(args...)
	return err
}

// securityOutput runs the keychain tool and returns its combined output.
func securityOutput(args ...string) (string, error) {
	cmd := exec.Command(securityPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(out))
		if detail == "" {
			return "", fmt.Errorf("security %s: %w", args[0], err)
		}
		return "", fmt.Errorf("security %s: %w: %s", args[0], err, detail)
	}
	return string(out), nil
}

// DefaultKeychain is the login keychain, which is where signing lands unless
// --ci asks for a throwaway one.
func DefaultKeychain() (string, error) {
	out, err := securityOutput("default-keychain", "-d", "user")
	if err != nil {
		return "", err
	}

	keychain := unquoteKeychain(out)
	if keychain == "" {
		return "", fmt.Errorf("could not determine the default keychain from %q", out)
	}

	return keychain, nil
}

// UserKeychains is the current user search list, in order.
func UserKeychains() ([]string, error) {
	out, err := securityOutput("list-keychains", "-d", "user")
	if err != nil {
		return nil, err
	}

	var keychains []string
	for _, line := range strings.Split(out, "\n") {
		if keychain := unquoteKeychain(line); keychain != "" {
			keychains = append(keychains, keychain)
		}
	}

	return keychains, nil
}

// CreateCIKeychain builds a keychain dedicated to this build and puts it at the
// front of the search list.
//
// It is recreated from scratch so that repeated builds on a persistent runner
// start clean, and it is created without an auto-lock timeout so that a long
// build cannot have it relock mid-way.
func CreateCIKeychain(path, password string) error {
	if _, err := os.Stat(path); err == nil {
		// A leftover from an earlier build; its password is not ours to guess.
		_ = security("delete-keychain", path)
	}

	if err := security("create-keychain", "-p", password, path); err != nil {
		return err
	}
	if err := security("set-keychain-settings", "-u", path); err != nil {
		return err
	}
	if err := security("unlock-keychain", "-p", password, path); err != nil {
		return err
	}

	existing, err := UserKeychains()
	if err != nil {
		return err
	}

	// Keep the other keychains so that anything else the build needs still
	// resolves, and do not list this one twice on a re-run.
	args := []string{"list-keychains", "-d", "user", "-s", path}
	for _, keychain := range existing {
		if !sameKeychain(keychain, path) {
			args = append(args, keychain)
		}
	}

	return security(args...)
}

// ImportIdentity puts a PKCS#12 file into a keychain.
//
// -T grants only codesign and security access to the key, rather than the
// blanket -A that would let any process use it. The password is passed on the
// command line because `security import` offers no other non-interactive way to
// supply it; it is random and discarded when the caller returns.
func ImportIdentity(p12Path, keychain, password string) error {
	return security(
		"import", p12Path,
		"-k", keychain,
		"-P", password,
		"-f", "pkcs12",
		"-T", "/usr/bin/codesign",
		"-T", securityPath,
	)
}

// SetKeyPartitionList stops codesign from raising an interactive "allow access"
// prompt the first time it uses an imported key. It needs the keychain
// password, so it only works on a keychain we created; on the login keychain
// the prompt has to be approved by hand, once.
func SetKeyPartitionList(keychain, password string) error {
	return security(
		"set-key-partition-list",
		"-S", "apple-tool:,apple:,codesign:",
		"-s", "-k", password,
		keychain,
	)
}

// WWDRInstalled reports whether the Apple Worldwide Developer Relations
// intermediate is present. Apple-issued certificates only chain to a trusted
// root when it is, and codesign's failure without it is an unhelpful "no
// identity found".
func WWDRInstalled() bool {
	_, err := securityOutput("find-certificate", "-a", "-c", "Apple Worldwide Developer Relations")
	return err == nil
}

// DefaultCIKeychainPath is where --ci installs when no path is given.
func DefaultCIKeychainPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating the home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Keychains", defaultKeychainName), nil
}

// unquoteKeychain trims the indentation and quoting `security` wraps keychain
// paths in.
func unquoteKeychain(line string) string {
	return strings.Trim(strings.TrimSpace(line), `"`)
}

// sameKeychain compares two keychain paths, allowing for the fact that
// `security` reports the resolved -db path for a keychain created without that
// suffix.
func sameKeychain(a, b string) bool {
	return a == b || a == b+"-db" || b == a+"-db"
}
