// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func signingDocumentWithProfile(name, uuid string) []byte {
	profile := "\x30\x82signed-data<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		`<plist version="1.0"><dict>` +
		`<key>Name</key><string>` + name + `</string>` +
		`<key>UUID</key><string>` + uuid + `</string>` +
		`</dict></plist>` + "\x00signature"
	return []byte(`{"profiles": {"` + name + `": {"uuid": "` + uuid + `", "state": "ACTIVE", "profile_content": "` +
		base64.StdEncoding.EncodeToString([]byte(profile)) + `"}}}`)
}

func TestSigningInstallReadsATerraformOutput(t *testing.T) {
	h := newHarness(t, &fakeConnect{})
	h.app.terraformOutput = func(dir, name string) ([]byte, error) {
		if dir != "infra" || name != "signing" {
			t.Errorf("read output %q in %q; want signing in infra", name, dir)
		}
		return signingDocumentWithProfile("App Store", "1234-ABCD"), nil
	}

	profiles := t.TempDir()
	if code := h.run("signing", "install", "--terraform", "infra", "--profiles-only", "--profile-dir", profiles); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if _, err := os.Stat(filepath.Join(profiles, "1234-ABCD.mobileprovision")); err != nil {
		t.Fatalf("the profile was not installed: %v", err)
	}
}

func TestSigningInstallNamesAnotherTerraformOutput(t *testing.T) {
	h := newHarness(t, &fakeConnect{})
	h.app.terraformOutput = func(_, name string) ([]byte, error) {
		if name != "ios_signing" {
			t.Errorf("read output %q; want ios_signing", name)
		}
		return signingDocumentWithProfile("App Store", "1234-ABCD"), nil
	}

	if code := h.run("signing", "install", "--terraform", ".", "--terraform-output", "ios_signing",
		"--profiles-only", "--profile-dir", t.TempDir()); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
}

func TestSigningInstallReportsTerraformsFailure(t *testing.T) {
	h := newHarness(t, &fakeConnect{})
	h.app.terraformOutput = func(string, string) ([]byte, error) {
		return nil, errors.New(`terraform output signing in infra: Output "signing" not found`)
	}

	if code := h.run("signing", "install", "--terraform", "infra", "--profiles-only", "--profile-dir", t.TempDir()); code != 1 {
		t.Fatalf("exit %d; want 1", code)
	}
	if !strings.Contains(h.errOut.String(), `Output "signing" not found`) {
		t.Fatalf("Terraform's own message was lost: %s", h.errOut)
	}
}

func TestSigningInstallTakesOneSource(t *testing.T) {
	for _, args := range [][]string{
		{"signing", "install", "--profiles-only"},
		{"signing", "install", "--profiles-only", "--from", "signing.json", "--terraform", "infra"},
	} {
		h := newHarness(t, &fakeConnect{})
		h.app.terraformOutput = func(string, string) ([]byte, error) {
			t.Error("terraform ran for a usage error")
			return nil, nil
		}
		if code := h.run(args...); code != 2 {
			t.Errorf("%v: exit %d; want 2", args, code)
		}
	}
}

// TestTerraformOutputRunsTerraform puts a stand-in terraform on PATH and checks
// the arguments it was given, since those are the whole contract.
func TestTerraformOutputRunsTerraform(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"$*\" = \"-chdir=infra output -no-color -json signing\" ]; then echo '{\"profiles\":{}}'; exit 0; fi\n" +
		"echo \"unexpected arguments: $*\" >&2; exit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "terraform"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	out, err := terraformOutputJSON("infra", "signing")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != `{"profiles":{}}` {
		t.Fatalf("output = %q", out)
	}

	if _, err := terraformOutputJSON("elsewhere", "signing"); err == nil || !strings.Contains(err.Error(), "unexpected arguments") {
		t.Fatalf("terraform's stderr was not reported: %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := terraformOutputJSON("infra", "signing"); err == nil || !strings.Contains(err.Error(), "--from") {
		t.Fatalf("a missing terraform should point at --from: %v", err)
	}
}
