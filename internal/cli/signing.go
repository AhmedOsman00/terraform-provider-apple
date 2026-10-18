// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newSigningCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signing",
		Short: "Install the certificates and profiles Terraform issued",
		Long: "The provider issues certificates and provisioning profiles; these commands put them\n" +
			"where Xcode and codesign look. The private keys never pass through Terraform: they are\n" +
			"read here, from a file, the environment, or an encrypted file in the repository.",
	}
	cmd.AddCommand(newSigningInstallCommand(a), newSigningCleanCommand(a), newSigningEncryptCommand(a))
	return cmd
}

type installResult struct {
	Keychain   string   `json:"keychain,omitempty"`
	Profiles   []string `json:"profiles"`
	Identities []string `json:"identities"`
	Skipped    []string `json:"skipped,omitempty"`
}

func newSigningInstallCommand(a *app) *cobra.Command {
	var (
		document         string
		keyArgs          []string
		keyDir           string
		keychainPath     string
		ephemeral        bool
		checkExpiry      int
		profilesOnly     bool
		certificatesOnly bool
		profileDirs      []string
	)

	cmd := &cobra.Command{
		Use:   "install --from FILE",
		Short: "Install provisioning profiles and signing identities from a signing document",
		Long: "Reads a signing document, which is the JSON of a Terraform output shaped like this:\n\n" +
			"  {\n" +
			"    \"certificates\": { \"<role>\": { \"certificate_content\": \"<base64 DER>\", \"expiration_date\": \"...\" } },\n" +
			"    \"profiles\": { \"<name>\": { \"uuid\": \"...\", \"state\": \"ACTIVE\", \"expiration_date\": \"...\",\n" +
			"                               \"profile_content\": \"<base64 .mobileprovision>\" } }\n" +
			"  }\n\n" +
			"profiles may also be a list of objects carrying \"name\". `terraform output -json <name>`\n" +
			"prints exactly that; --from - reads it from stdin.\n\n" +
			"Every profile is installed under its UUID, replacing older copies of the same profile.\n" +
			"Every certificate whose private key is found is imported as a signing identity. A role's\n" +
			"key is the first of:\n\n" +
			"  1. --key ROLE=PATH\n" +
			"  2. APPLE_SIGNING_<ROLE>_KEY, the PEM itself or base64 of it\n" +
			"  3. KEY_DIR/<role>.key\n" +
			"  4. KEY_DIR/<role>.key.enc, decrypted with APPLE_SIGNING_PASSPHRASE or a prompt\n\n" +
			"--ephemeral-keychain creates a keychain nothing else uses, adds Apple's WWDR\n" +
			"intermediate, imports into it and lets codesign use the keys without a prompt: what a\n" +
			"CI runner needs. `apple signing clean` removes it.",
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "darwin" && !profilesOnly {
				return usageError("installing identities needs macOS's keychain; pass --profiles-only elsewhere")
			}
			if profilesOnly && certificatesOnly {
				return usageError("--profiles-only and --certificates-only exclude each other")
			}
			if document == "" {
				return usageError("--from names the signing document, or - for stdin")
			}
			keys, err := parseKeyArgs(keyArgs)
			if err != nil {
				return err
			}

			data, err := readInput(cmd.InOrStdin(), document)
			if err != nil {
				return err
			}
			doc, err := parseSigningDocument(data)
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("check-expiry") {
				if err := checkCertificateExpiry(a, doc, checkExpiry); err != nil {
					return err
				}
			}

			result := installResult{Profiles: []string{}, Identities: []string{}}
			if !certificatesOnly {
				if len(profileDirs) == 0 {
					profileDirs = defaultProfileDirs()
				}
				installed, skipped, err := installProfiles(a, doc.Profiles, profileDirs)
				if err != nil {
					return err
				}
				result.Profiles, result.Skipped = installed, skipped
			}

			if !profilesOnly {
				if ephemeral && keychainPath == "" {
					keychainPath = ephemeralKeychainPath()
				}
				if keychainPath == "" {
					keychainPath = loginKeychain()
				}
				k := keychain{path: keychainPath}
				result.Keychain = keychainPath

				password := ""
				if ephemeral {
					if password, err = k.create(); err != nil {
						return err
					}
					a.logf("Created %s.", keychainPath)
				}

				resolver := &keyResolver{a: a, flags: keys, dir: keyDir}
				imported, skipped, err := installIdentities(a, k, doc.Certificates, resolver)
				if err != nil {
					return err
				}
				result.Identities = imported
				result.Skipped = append(result.Skipped, skipped...)

				if ephemeral {
					if err := k.allowCodesign(password); err != nil {
						return err
					}
				} else if !k.hasWWDR() {
					a.logf("The keychain has no Apple WWDR intermediate, so the identities will show as untrusted. "+
						"Get it from %s and `security import` it.", wwdrURL)
				}
				if identities := k.identities(); identities != "" {
					a.logf("%s", identities)
				}
			}

			return a.emit(result, result.Keychain)
		},
	}

	cmd.Flags().StringVar(&document, "from", "", "the signing document, or - for stdin")
	cmd.Flags().StringArrayVar(&keyArgs, "key", nil, "ROLE=PATH, the private key for ROLE; repeatable")
	cmd.Flags().StringVar(&keyDir, "key-dir", "", "a directory of <role>.key or <role>.key.enc files")
	cmd.Flags().StringVar(&keychainPath, "keychain", "", "the keychain to import into (default: login, or a temporary one with --ephemeral-keychain)")
	cmd.Flags().BoolVar(&ephemeral, "ephemeral-keychain", false, "create a throwaway keychain for a CI runner")
	cmd.Flags().IntVar(&checkExpiry, "check-expiry", 0, "warn about a certificate expiring within DAYS, and stop on an expired one")
	cmd.Flags().BoolVar(&profilesOnly, "profiles-only", false, "skip the certificates")
	cmd.Flags().BoolVar(&certificatesOnly, "certificates-only", false, "skip the profiles")
	cmd.Flags().StringArrayVar(&profileDirs, "profile-dir", nil, "where to install profiles, repeatable (default: the two directories Xcode reads)")
	return cmd
}

func newSigningCleanCommand(a *app) *cobra.Command {
	var keychainPath string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove the keychain --ephemeral-keychain created",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if keychainPath == "" {
				keychainPath = ephemeralKeychainPath()
			}
			if keychainPath == loginKeychain() {
				return usageError("refusing to delete the login keychain")
			}
			removed, err := keychain{path: keychainPath}.remove()
			if err != nil {
				return err
			}
			if removed {
				a.logf("Deleted %s.", keychainPath)
			} else {
				a.logf("No keychain at %s.", keychainPath)
			}
			return a.emit(map[string]any{"keychain": keychainPath, "removed": removed}, keychainPath)
		},
	}

	cmd.Flags().StringVar(&keychainPath, "keychain", "", "the keychain to remove (default: the one --ephemeral-keychain creates)")
	return cmd
}

func newSigningEncryptCommand(a *app) *cobra.Command {
	var keyDir string

	cmd := &cobra.Command{
		Use:   "encrypt --key-dir DIR",
		Short: "Encrypt every DIR/<role>.key into a <role>.key.enc that can be committed",
		Long: "Encrypts each private key with APPLE_SIGNING_PASSPHRASE, or a passphrase asked for twice;\n" +
			"an empty answer generates one. Existing .key.enc files must open with the same passphrase,\n" +
			"so a team keeps a single secret. The format is `openssl enc -aes-256-cbc -pbkdf2 -iter\n" +
			"600000 -a`, so OpenSSL can open the files too.",
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if keyDir == "" {
				return usageError("--key-dir names the directory holding <role>.key (or APPLE_KEY_DIR)")
			}
			plainKeys, _ := filepath.Glob(filepath.Join(keyDir, "*.key"))
			if len(plainKeys) == 0 {
				return fmt.Errorf("no .key files in %s to encrypt", keyDir)
			}

			passphrase, err := newPassphrase(a)
			if err != nil {
				return err
			}

			existing, _ := filepath.Glob(filepath.Join(keyDir, "*.key.enc"))
			for _, path := range existing {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if _, err := decryptKey(data, passphrase); err != nil {
					return fmt.Errorf("%s was encrypted with a different passphrase; use that one, so the team keeps a single secret",
						filepath.Base(path))
				}
			}

			written := []string{}
			for _, path := range plainKeys {
				plain, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if _, err := parsePrivateKey(plain); err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				sealed, err := encryptKey(plain, passphrase)
				if err != nil {
					return err
				}
				if check, err := decryptKey(sealed, passphrase); err != nil || !bytes.Equal(check, plain) {
					return fmt.Errorf("encrypting %s did not round-trip", path)
				}
				if err := os.WriteFile(path+".enc", sealed, 0o644); err != nil {
					return err
				}
				a.logf("Wrote %s.enc", filepath.Base(path))
				written = append(written, path+".enc")
			}
			return a.emit(map[string]any{"written": written}, strings.Join(written, "\n"))
		},
	}

	cmd.Flags().StringVar(&keyDir, "key-dir", "", "the directory holding <role>.key")
	return cmd
}

func parseKeyArgs(args []string) (map[string]string, error) {
	keys := map[string]string{}
	for _, arg := range args {
		role, path, ok := strings.Cut(arg, "=")
		if !ok || role == "" || path == "" {
			return nil, usageError("--key takes ROLE=PATH, not %q", arg)
		}
		keys[role] = path
	}
	return keys, nil
}

func readInput(stdin io.Reader, path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, usageError("no file %s", path)
	}
	return data, nil
}

func checkCertificateExpiry(a *app, doc *signingDocument, days int) error {
	roles := sortedKeys(doc.Certificates)
	expired := false
	for _, role := range roles {
		expires, err := parseAppleDate(doc.Certificates[role].ExpirationDate)
		if err != nil {
			return fmt.Errorf("the %s certificate: %w", role, err)
		}
		left := int(expires.Sub(a.now()).Hours() / 24)
		switch {
		case expires.Before(a.now()):
			a.logf("The %s certificate expired on %s. Apply Terraform to renew it.", role, expires.Format("2006-01-02"))
			expired = true
		case left < days:
			a.logf("The %s certificate expires in %d days (%s). Apply Terraform to renew it.", role, left, expires.Format("2006-01-02"))
		default:
			a.logf("The %s certificate is good for %d more days.", role, left)
		}
	}
	if expired {
		return errors.New("a certificate has expired")
	}
	return nil
}

func defaultProfileDirs() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles"),
		filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles"),
	}
}

// installProfiles writes each active profile as <uuid>.mobileprovision, which
// is how Xcode indexes them, and removes older copies carrying the same name.
func installProfiles(a *app, profiles []signingProfile, dirs []string) (installed, skipped []string, err error) {
	installed = []string{}
	if len(profiles) == 0 {
		a.logf("No profiles in the document.")
		return installed, nil, nil
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, err
		}
	}

	for _, profile := range profiles {
		if profile.State != "" && profile.State != "ACTIVE" {
			a.logf("Skipping %s: Apple reports it %s.", profile.Name, profile.State)
			skipped = append(skipped, profile.Name)
			continue
		}
		data, err := base64.StdEncoding.DecodeString(profile.ProfileContent)
		if err != nil {
			return nil, nil, fmt.Errorf("profile %s: profile_content is not base64", profile.Name)
		}
		name, uuid, err := profilePlist(data)
		if err != nil {
			return nil, nil, fmt.Errorf("profile %s: %w", profile.Name, err)
		}
		if (profile.Name != "" && name != profile.Name) || (profile.UUID != "" && uuid != profile.UUID) {
			a.logf("Skipping %s: the payload decodes to %s (%s).", profile.Name, name, uuid)
			skipped = append(skipped, profile.Name)
			continue
		}

		for _, dir := range dirs {
			others, _ := filepath.Glob(filepath.Join(dir, "*.mobileprovision"))
			for _, other := range others {
				if strings.TrimSuffix(filepath.Base(other), ".mobileprovision") == uuid {
					continue
				}
				content, err := os.ReadFile(other)
				if err != nil {
					continue
				}
				if otherName, _, err := profilePlist(content); err == nil && otherName == name {
					if err := os.Remove(other); err == nil {
						a.logf("Removed stale %s (%s).", name, filepath.Base(other))
					}
				}
			}
			if err := os.WriteFile(filepath.Join(dir, uuid+".mobileprovision"), data, 0o644); err != nil {
				return nil, nil, err
			}
		}
		a.logf("Installed %s (%s).", name, uuid)
		installed = append(installed, name)
	}
	return installed, skipped, nil
}

func installIdentities(a *app, k keychain, certificates map[string]signingCertificate, resolver *keyResolver) (imported, skipped []string, err error) {
	imported = []string{}
	if len(certificates) == 0 {
		a.logf("No certificates in the document.")
		return imported, nil, nil
	}

	for _, role := range sortedKeys(certificates) {
		der, err := base64.StdEncoding.DecodeString(certificates[role].CertificateContent)
		if err != nil {
			return nil, nil, fmt.Errorf("the %s certificate: certificate_content is not base64", role)
		}
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, nil, fmt.Errorf("the %s certificate: %w", role, err)
		}
		subject := certificate.Subject.CommonName

		if k.hasCertificate(certificate) {
			a.logf("Already in the keychain: %s.", subject)
			imported = append(imported, subject)
			continue
		}

		key, source, err := resolver.resolve(role)
		if err != nil {
			return nil, nil, err
		}
		if key == nil {
			a.logf("Skipping %s (%s): no key. See --help for where keys are looked up.", role, subject)
			skipped = append(skipped, role)
			continue
		}
		if !keyMatches(certificate, key) {
			a.logf("Skipping %s: the key from %s does not match the issued certificate. Was the CSR regenerated after the apply?", role, source)
			skipped = append(skipped, role)
			continue
		}

		if err := k.importIdentity(certificate, key); err != nil {
			return nil, nil, err
		}
		a.logf("Imported %s (key from %s), expires %s.", subject, source, certificate.NotAfter.Format("2006-01-02"))
		imported = append(imported, subject)
	}
	return imported, skipped, nil
}

// keyResolver finds a role's private key. Decrypted keys only ever live in
// memory.
type keyResolver struct {
	a          *app
	flags      map[string]string
	dir        string
	passphrase string
}

func (r *keyResolver) resolve(role string) (crypto.Signer, string, error) {
	if path, ok := r.flags[role]; ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("--key %s=%s: %w", role, path, err)
		}
		key, err := parsePrivateKey(data)
		return key, "--key " + path, err
	}

	env := "APPLE_SIGNING_" + strings.ToUpper(role) + "_KEY"
	if value := os.Getenv(env); value != "" {
		data := []byte(value)
		if !strings.Contains(value, "-----BEGIN") {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
			if err != nil {
				return nil, "", fmt.Errorf("%s is neither a PEM key nor base64 of one", env)
			}
			data = decoded
		}
		key, err := parsePrivateKey(data)
		return key, "$" + env, err
	}

	if r.dir == "" {
		return nil, "", nil
	}

	plain := filepath.Join(r.dir, role+".key")
	if data, err := os.ReadFile(plain); err == nil {
		key, err := parsePrivateKey(data)
		return key, plain, err
	}

	sealed := filepath.Join(r.dir, role+".key.enc")
	data, err := os.ReadFile(sealed)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if r.passphrase == "" {
		if r.passphrase, err = passphrase(r.a, "Passphrase for the signing keys: "); err != nil {
			return nil, "", fmt.Errorf("%s needs APPLE_SIGNING_PASSPHRASE: %w", sealed, err)
		}
	}
	plainKey, err := decryptKey(data, r.passphrase)
	if err != nil {
		return nil, "", fmt.Errorf("could not decrypt %s: %w", sealed, err)
	}
	key, err := parsePrivateKey(plainKey)
	return key, sealed, err
}

func passphrase(a *app, prompt string) (string, error) {
	if value := os.Getenv("APPLE_SIGNING_PASSPHRASE"); value != "" {
		return value, nil
	}
	return ask(a, prompt)
}

func ask(a *app, prompt string) (string, error) {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return "", errors.New("there is no terminal to ask on")
	}
	defer tty.Close()

	fmt.Fprint(a.errOut, prompt)
	answer, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(a.errOut)
	return string(answer), err
}

// newPassphrase is the one to encrypt with: the environment's, or one typed
// twice, or a generated one when the answer is empty.
func newPassphrase(a *app) (string, error) {
	if value := os.Getenv("APPLE_SIGNING_PASSPHRASE"); value != "" {
		return value, nil
	}
	first, err := ask(a, "Passphrase (empty generates one): ")
	if err != nil {
		return "", fmt.Errorf("set APPLE_SIGNING_PASSPHRASE: %w", err)
	}
	if first == "" {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return "", err
		}
		generated := base64.StdEncoding.EncodeToString(secret)
		a.logf("Generated passphrase; store it in the team's password manager and CI secrets:\n  %s", generated)
		return generated, nil
	}
	second, err := ask(a, "Again: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("the passphrases differ")
	}
	return first, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
