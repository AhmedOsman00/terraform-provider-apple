// Command applesign installs Apple code-signing assets onto a machine, so that
// xcodebuild can sign.
//
// Terraform manages the Apple developer portal; it deliberately does not touch
// the local keychain or the provisioning profile directory. This command covers
// that last mile, which is the part `fastlane match` does with its shared
// encrypted repository.
//
// It reads a signing bundle as JSON and knows nothing about where that JSON
// came from -- no backend, no secret store -- so every source composes as a
// pipe:
//
//	terraform -chdir=examples/signing output -json signing_bundle | applesign install -
//	sops -d signing/bundle.enc.json | applesign install -
//	op read "op://Eng/ios-signing/bundle" | applesign install -
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// version is overridden at build time for released binaries, matching the
// provider's own versioning.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "applesign: error: %v\n", err)
		os.Exit(1)
	}
}

// run dispatches a command line, returning an error rather than exiting so that
// it stays testable.
func run(args []string) error {
	if len(args) == 0 {
		usage(os.Stderr)
		return errors.New("no command given")
	}

	switch args[0] {
	case "install":
		return runInstall(args[1:])
	case "version", "-version", "--version":
		fmt.Printf("applesign %s\n", version)
		return nil
	case "help", "-h", "-help", "--help":
		usage(os.Stdout)
		return nil
	default:
		usage(os.Stderr)
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

// usage documents the command line.
func usage(w io.Writer) {
	fmt.Fprint(w, `Usage: applesign <command> [options]

Installs the certificates and provisioning profiles described by a signing
bundle: identities into a keychain, profiles where Xcode looks for them.

Commands:
  install [options] <source>   Install the signing assets in a bundle
  version                      Print the applesign version
  help                         Show this help

Install options:
  --ci             Install into a dedicated keychain unlocked for this session,
                   instead of the login keychain. Use this on build runners.
  --keychain PATH  Keychain to create and use with --ci
                   (default: $HOME/Library/Keychains/apple-signing.keychain-db)
  --dry-run        Report what would be installed without changing anything

The source is "-" for stdin, or a path. applesign knows nothing about where the
bundle is stored, so any source composes as a pipe:

  terraform -chdir=examples/signing output -json signing_bundle | applesign install -
  sops -d signing/bundle.enc.json | applesign install -
  op read "op://Eng/ios-signing/bundle" | applesign install -

Environment:
  APPLE_SIGNING_KEYCHAIN_PASSWORD
      Password for the --ci keychain. Generated randomly when unset, which is
      what you want unless a later build step needs to unlock it again.
`)
}
