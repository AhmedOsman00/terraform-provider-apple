package main

import (
	"strings"
	"testing"
)

func TestRunInstallArgumentErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args         []string
		wantContains string
	}{
		"no source":            {args: nil, wantContains: "needs a source"},
		"two sources":          {args: []string{"-", "other.json"}, wantContains: "exactly one source"},
		"--keychain sans --ci": {args: []string{"--keychain", "/tmp/x.keychain-db", "-"}, wantContains: "only applies with --ci"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := runInstall(test.args)
			if err == nil {
				t.Fatal("runInstall() succeeded, want an error")
			}
			if !strings.Contains(err.Error(), test.wantContains) {
				t.Errorf("runInstall() error = %v, want it to contain %q", err, test.wantContains)
			}
		})
	}
}

func TestRunUnknownCommand(t *testing.T) {
	t.Parallel()

	if err := run([]string{"nuke"}); err == nil {
		t.Fatal("run() succeeded on an unknown command")
	}

	if err := run(nil); err == nil {
		t.Fatal("run() succeeded with no command")
	}

	if err := run([]string{"help"}); err != nil {
		t.Errorf("run(help) error = %v", err)
	}
	if err := run([]string{"version"}); err != nil {
		t.Errorf("run(version) error = %v", err)
	}
}

// Go's flag package stops parsing at the first non-flag argument, so a flag
// written after the source would otherwise be swallowed as a second source --
// and `install - --ci` would quietly install into the login keychain.
func TestRunInstallAcceptsFlagsAfterSource(t *testing.T) {
	t.Parallel()

	// The bundle is unreadable either way; what matters is that --keychain was
	// seen, which only its own validation error can prove.
	err := runInstall([]string{"/nonexistent/bundle.json", "--keychain", "/tmp/x.keychain-db"})
	if err == nil {
		t.Fatal("runInstall() succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "only applies with --ci") {
		t.Errorf("runInstall() error = %v, want the --keychain validation error", err)
	}
}
