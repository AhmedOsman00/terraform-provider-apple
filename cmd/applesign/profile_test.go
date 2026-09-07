// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Xcode 16's directory is only worth creating on a machine that has Xcode; the
// classic one is read by every version and is always installed to.
func TestProfileDirectories(t *testing.T) {
	t.Parallel()

	classic := filepath.Join("Library", "MobileDevice", "Provisioning Profiles")
	xcode16 := filepath.Join("Library", "Developer", "Xcode", "UserData", "Provisioning Profiles")

	t.Run("without Xcode", func(t *testing.T) {
		t.Parallel()

		home := t.TempDir()
		got := ProfileDirectories(home)

		want := []string{filepath.Join(home, classic)}
		if len(got) != 1 || got[0] != want[0] {
			t.Errorf("ProfileDirectories() = %v, want %v", got, want)
		}
	})

	t.Run("with Xcode", func(t *testing.T) {
		t.Parallel()

		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, "Library", "Developer", "Xcode"), 0o700); err != nil {
			t.Fatalf("creating the Xcode directory: %v", err)
		}

		got := ProfileDirectories(home)
		want := []string{filepath.Join(home, classic), filepath.Join(home, xcode16)}

		if len(got) != len(want) {
			t.Fatalf("ProfileDirectories() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("ProfileDirectories()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestInstallProfile(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "Library", "Developer", "Xcode"), 0o700); err != nil {
		t.Fatalf("creating the Xcode directory: %v", err)
	}

	directories := ProfileDirectories(home)
	content := []byte("a provisioning profile")
	uuid := "11111111-2222-3333-4444-555555555555"

	written, err := InstallProfile(content, uuid, directories)
	if err != nil {
		t.Fatalf("InstallProfile() error = %v", err)
	}
	if len(written) != len(directories) {
		t.Fatalf("InstallProfile() wrote to %d directories, want %d", len(written), len(directories))
	}

	for _, directory := range directories {
		path := filepath.Join(directory, uuid+".mobileprovision")

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if string(got) != string(content) {
			t.Errorf("%s content = %q, want %q", path, got, content)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s mode = %v, want 0600", path, perm)
		}
	}

	// Reinstalling replaces the profile rather than failing, which is what a
	// second run after a rotation does.
	if _, err := InstallProfile([]byte("replaced"), uuid, directories); err != nil {
		t.Fatalf("InstallProfile() on a second run error = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(directories[0], uuid+".mobileprovision"))
	if err != nil {
		t.Fatalf("reading the replaced profile: %v", err)
	}
	if string(got) != "replaced" {
		t.Errorf("content after reinstall = %q, want %q", got, "replaced")
	}
}
