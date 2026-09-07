package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// installOptions is the parsed `applesign install` command line.
type installOptions struct {
	source   string
	ci       bool
	dryRun   bool
	keychain string
}

// keychainPasswordEnv supplies the --ci keychain password when a later build
// step needs to unlock it again. Unset is the usual case, and then the password
// is generated and discarded.
const keychainPasswordEnv = "APPLE_SIGNING_KEYCHAIN_PASSWORD"

// runInstall parses the install command line and installs the bundle it names.
func runInstall(args []string) error {
	var opts installOptions

	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { usage(os.Stderr) }
	fs.BoolVar(&opts.ci, "ci", false, "install into a dedicated keychain unlocked for this session")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "report what would be installed without changing anything")
	fs.StringVar(&opts.keychain, "keychain", "", "keychain to create and use with --ci")

	// Parsing resumes after each positional argument, so that `install - --ci`
	// means what it looks like. Go's flag package stops at the first
	// non-flag, and silently installing into the login keychain because --ci
	// came last is the wrong way to learn that.
	var positional []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if rest = fs.Args(); len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}

	switch len(positional) {
	case 1:
		opts.source = positional[0]
	case 0:
		return errors.New(`install needs a source: "-" reads the bundle from stdin`)
	default:
		return errors.New("install takes exactly one source")
	}

	if opts.keychain != "" && !opts.ci {
		return errors.New("--keychain only applies with --ci; without it, signing goes into the login keychain")
	}

	bundle, err := readBundle(opts.source)
	if err != nil {
		return err
	}

	return install(bundle, opts)
}

// install performs the whole last mile: identities into a keychain, profiles
// into the directories Xcode reads.
func install(bundle *Bundle, opts installOptions) error {
	if !opts.dryRun && runtime.GOOS != "darwin" {
		return errors.New("signing assets can only be installed on macOS; --dry-run inspects a bundle anywhere")
	}

	section("Signing bundle")
	info("bundle identifier: %s", bundle.BundleIdentifier)
	info("certificates:      %d", len(bundle.Certificates))
	info("profiles:          %d", len(bundle.Profiles))

	// Every identity below is decoded before anything is installed, so that a
	// malformed bundle fails without leaving the keychain half-populated.
	identities := make([]*Identity, len(bundle.Certificates))
	for i, certificate := range bundle.Certificates {
		identity, err := NewIdentity(certificate.CertificateContent, certificate.PrivateKeyPEM)
		if err != nil {
			return fmt.Errorf("%s: %w", certificate.label(), err)
		}
		identities[i] = identity
	}

	keychain, keychainPassword, err := prepareKeychain(opts)
	if err != nil {
		return err
	}

	if err := installCertificates(bundle, identities, keychain, keychainPassword, opts); err != nil {
		return err
	}

	if err := installProfiles(bundle, opts); err != nil {
		return err
	}

	summarize(bundle, keychain, opts)

	return nil
}

// prepareKeychain reports which keychain to install into, creating a throwaway
// one first under --ci. The returned password is empty for the login keychain,
// whose password we neither know nor want.
func prepareKeychain(opts installOptions) (keychain, password string, err error) {
	if opts.dryRun {
		section("Dry run -- nothing will be installed")
		return "", "", nil
	}

	if !opts.ci {
		keychain, err = DefaultKeychain()
		if err != nil {
			return "", "", err
		}
		section("Installing into the login keychain %s", keychain)
		return keychain, "", nil
	}

	keychain = opts.keychain
	if keychain == "" {
		if keychain, err = DefaultCIKeychainPath(); err != nil {
			return "", "", err
		}
	}

	password = os.Getenv(keychainPasswordEnv)
	if password == "" {
		if password, err = randomPassword(); err != nil {
			return "", "", err
		}
	}

	section("Preparing CI keychain %s", keychain)
	if err := CreateCIKeychain(keychain, password); err != nil {
		return "", "", err
	}
	info("keychain created and unlocked")

	return keychain, password, nil
}

// installCertificates assembles a PKCS#12 container per identity and imports it.
func installCertificates(bundle *Bundle, identities []*Identity, keychain, keychainPassword string, opts installOptions) error {
	section("Certificates")

	// Key material only ever reaches disk inside this directory, which is
	// owner-only and removed on every exit path.
	workdir, err := os.MkdirTemp("", "apple-signing")
	if err != nil {
		return fmt.Errorf("creating a working directory: %w", err)
	}
	defer os.RemoveAll(workdir) //nolint:errcheck // best effort cleanup.

	for i, certificate := range bundle.Certificates {
		identity := identities[i]

		fmt.Printf("\n  %s (%s)\n", certificate.label(), certificate.CertificateType)
		info("identity: %s", identity.CommonName())
		info("team:     %s", orNotPresent(identity.TeamID()))
		info("expires:  %s", orNotPresent(certificate.ExpirationDate))

		if opts.dryRun {
			continue
		}

		password, err := randomPassword()
		if err != nil {
			return err
		}

		p12, err := identity.PKCS12(password)
		if err != nil {
			return fmt.Errorf("%s: %w", certificate.label(), err)
		}

		path := filepath.Join(workdir, certificate.label()+".p12")
		if err := os.WriteFile(path, p12, 0o600); err != nil {
			return fmt.Errorf("writing the PKCS#12 container: %w", err)
		}

		if err := ImportIdentity(path, keychain, password); err != nil {
			return fmt.Errorf("%s: %w", certificate.label(), err)
		}

		info("imported into %s", keychain)
	}

	if opts.dryRun {
		return nil
	}

	if opts.ci {
		if err := SetKeyPartitionList(keychain, keychainPassword); err != nil {
			info("warning: could not set the key partition list; codesign may prompt: %v", err)
		}
	}

	if !WWDRInstalled() {
		section("Warning: Apple WWDR intermediate certificate not found")
		info("Signing identities will not be trusted until it is installed.")
		info("Download it from https://www.apple.com/certificateauthority/ and run:")
		info("  security import AppleWWDRCAG3.cer -k %q", keychain)
	}

	return nil
}

// installProfiles decodes each provisioning profile and drops it where Xcode
// looks for it.
func installProfiles(bundle *Bundle, opts installOptions) error {
	if len(bundle.Profiles) == 0 {
		section("No provisioning profiles to install")
		return nil
	}

	section("Provisioning profiles")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locating the home directory: %w", err)
	}
	directories := ProfileDirectories(home)

	for _, profile := range bundle.Profiles {
		fmt.Printf("\n  %s\n", profile.Name)
		info("type:    %s", profile.ProfileType)
		info("uuid:    %s", orNotPresent(profile.UUID))
		info("expires: %s", orNotPresent(profile.ExpirationDate))

		if opts.dryRun {
			continue
		}

		// Xcode indexes profiles by the UUID in the filename, so a profile
		// without one cannot be installed where Xcode will find it.
		if profile.UUID == "" {
			info("warning: no UUID recorded, skipping")
			continue
		}

		content, err := decodeContent(profile.ProfileContent)
		if err != nil {
			return fmt.Errorf("%s: decoding profile_content as base64: %w", profile.Name, err)
		}

		written, err := InstallProfile(content, profile.UUID, directories)
		if err != nil {
			return fmt.Errorf("%s: %w", profile.Name, err)
		}

		for _, directory := range written {
			info("installed to %s", directory)
		}
	}

	return nil
}

// summarize prints the build settings that make xcodebuild use what was just
// installed.
func summarize(bundle *Bundle, keychain string, opts installOptions) {
	section("Done")

	if opts.dryRun {
		info("dry run: no keychain or profile changes were made")
		return
	}

	info("Set these in your Xcode build settings or xcodebuild invocation:")
	fmt.Printf("\n  PRODUCT_BUNDLE_IDENTIFIER = %s\n", bundle.BundleIdentifier)
	fmt.Printf("  CODE_SIGN_STYLE           = Manual\n")
	for _, profile := range bundle.Profiles {
		fmt.Printf("  PROVISIONING_PROFILE_SPECIFIER = %s   # %s\n", profile.Name, profile.ProfileType)
	}

	if opts.ci {
		fmt.Println()
		info("Keychain %s is unlocked for this session.", keychain)
		if os.Getenv(keychainPasswordEnv) == "" {
			info("Its password was generated and discarded; re-run applesign to unlock it again.")
		}
	}
}

// section prints a heading.
func section(format string, args ...any) {
	fmt.Printf("\n==> "+format+"\n", args...)
}

// info prints an indented detail line.
func info(format string, args ...any) {
	fmt.Printf("  "+format+"\n", args...)
}

// orNotPresent keeps a missing optional field from printing as an empty column.
func orNotPresent(value string) string {
	if value == "" {
		return "(not recorded)"
	}
	return value
}
