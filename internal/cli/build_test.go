// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"howett.net/plist"
)

const fixtureExecutable = "\xcf\xfa\xed\xfe not really a Mach-O"

func fixtureInfo(version, build string) []byte {
	data, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleShortVersionString": version,
		"CFBundleVersion":            build,
		"CFBundleExecutable":         "App",
	}, plist.BinaryFormat)
	if err != nil {
		panic(err)
	}
	return data
}

// writeFixtureIPA writes an .ipa the way Xcode lays one out: a binary
// Info.plist and the executable under Payload/App.app.
func writeFixtureIPA(t *testing.T, version, build string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "App.ipa")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, content := range map[string][]byte{
		"Payload/App.app/Info.plist":                  fixtureInfo(version, build),
		"Payload/App.app/App":                         []byte(fixtureExecutable),
		"Payload/App.app/PlugIns/Widget.appex/Widget": []byte("a nested bundle"),
	} {
		w, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeFixtureArchive writes an .xcarchive holding a macOS-layout app, to
// cover the Contents/ form as well.
func writeFixtureArchive(t *testing.T, version, build string) string {
	t.Helper()
	archive := filepath.Join(t.TempDir(), "App.xcarchive")
	contents := filepath.Join(archive, "Products", "Applications", "App.app", "Contents")
	if err := os.MkdirAll(filepath.Join(contents, "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), fixtureInfo(version, build), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contents, "MacOS", "App"), []byte(fixtureExecutable), 0o755); err != nil {
		t.Fatal(err)
	}
	return archive
}

// signedWith stands in for codesign, checking it was handed the executable.
func signedWith(t *testing.T, entitlements map[string]any) func(string) (map[string]any, error) {
	return func(executable string) (map[string]any, error) {
		content, err := os.ReadFile(executable)
		if err != nil || string(content) != fixtureExecutable {
			t.Errorf("the entitlements were read from %s, which is not the app's executable", executable)
		}
		return entitlements, nil
	}
}

func TestBuildVerifyPassesOnAMatchingIPA(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{})
	h.app.entitlements = signedWith(t, map[string]any{
		"aps-environment":                        "production",
		"get-task-allow":                         false,
		"com.apple.developer.associated-domains": []any{"applinks:example.com", "webcredentials:example.com"},
	})
	code := h.run("build", "verify", writeFixtureIPA(t, "1.2.0", "7"),
		"--bundle-id", "com.example.app", "--version", "1.2.0", "--build", "7",
		"--entitlement", "aps-environment=production",
		"--entitlement", "get-task-allow=false",
		"--entitlement", "com.apple.developer.associated-domains=applinks:example.com")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if got := h.out.String(); got != "ok\n" {
		t.Errorf("stdout = %q", got)
	}
}

func TestBuildVerifyReportsEveryMismatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{})
	h.app.entitlements = signedWith(t, map[string]any{"aps-environment": "development"})
	code := h.run("build", "verify", writeFixtureIPA(t, "1.2.0", "7"),
		"--version", "1.3.0", "--build", "8",
		"--entitlement", "aps-environment=production",
		"--entitlement", "com.apple.developer.icloud-services=CloudKit")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	for _, want := range []string{
		"CFBundleShortVersionString is 1.2.0, expected 1.3.0",
		"CFBundleVersion is 7, expected 8",
		"aps-environment is development, expected production",
		"not signed with com.apple.developer.icloud-services",
	} {
		if !strings.Contains(h.errOut.String(), want) {
			t.Errorf("stderr is missing %q:\n%s", want, h.errOut)
		}
	}
}

func TestBuildVerifyReadsAnArchiveWithoutCodesign(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{})
	h.app.entitlements = func(string) (map[string]any, error) {
		t.Error("entitlements were read although none were asked for")
		return nil, nil
	}
	code := h.run("build", "verify", writeFixtureArchive(t, "2.0.0", "1"), "--bundle-id", "com.example.app",
		"--version", "2.0.0", "--build", "1", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	var result verifyResult
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.BundleID != "com.example.app" || result.Version != "2.0.0" || result.Build != "1" {
		t.Errorf("result = %+v", result)
	}
}

func TestBuildVerifyPrintsJSONOnFailure(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{})
	if code := h.run("build", "verify", writeFixtureIPA(t, "1.2.0", "7"), "--build", "9", "--json"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	var result verifyResult
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, h.out)
	}
	if len(result.Problems) != 1 {
		t.Errorf("problems = %v", result.Problems)
	}
}

func TestBuildVerifyRefusesWhatIsNotABuild(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{})
	notes := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notes, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run("build", "verify", notes); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if code := h.run("build", "verify", writeFixtureIPA(t, "1.2.0", "7"), "--entitlement", "aps-environment"); code != 2 {
		t.Fatalf("exit %d for an entitlement without a value, want 2", code)
	}
}

// TestBuildVerifyVersionReadsItsVariable covers a command's own --version,
// which is not the root's and so reads APPLE_VERSION like any other flag.
func TestBuildVerifyVersionReadsItsVariable(t *testing.T) {
	t.Setenv("APPLE_VERSION", "9.9.9")

	h := newHarness(t, &fakeConnect{})
	if code := h.run("build", "verify", writeFixtureIPA(t, "1.2.0", "7")); code != 1 {
		t.Fatalf("exit %d, want 1 for a version APPLE_VERSION does not match", code)
	}
	if !strings.Contains(h.errOut.String(), "expected 9.9.9") {
		t.Errorf("stderr = %s", h.errOut)
	}
}

// uploadHarness serves the parts of an upload from a TLS server of its own,
// since parts are only ever sent over HTTPS.
func uploadHarness(t *testing.T, fake *fakeConnect) (*harness, *[]byte) {
	t.Helper()
	var (
		mu       sync.Mutex
		received []byte
	)
	parts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		received, _ = io.ReadAll(r.Body)
	}))
	t.Cleanup(parts.Close)
	fake.partsURL = parts.URL + "/part"

	h := newHarness(t, fake)
	newClient := h.app.newClient
	h.app.newClient = func() (*apple.Client, error) {
		client, err := newClient()
		if err == nil {
			client.UploadHTTPClient = parts.Client()
		}
		return client, err
	}
	return h, &received
}

func TestBuildUploadSendsTheIPAAndWaitsForApple(t *testing.T) {
	t.Parallel()

	fake := &fakeConnect{uploadStates: []string{
		`{"state":"PROCESSING"}`,
		`{"state":"COMPLETE","warnings":[{"code":"90725","description":"SDK version issue"}]}`,
	}}
	h, received := uploadHarness(t, fake)
	ipa := writeFixtureIPA(t, "1.2.0", "7")

	if code := h.run("build", "upload", ipa, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	content, _ := os.ReadFile(ipa)
	if string(*received) != string(content) {
		t.Errorf("the part carried %d bytes, want the %d-byte .ipa", len(*received), len(content))
	}
	if h.sleeps != 1 {
		t.Errorf("slept %d times, want 1", h.sleeps)
	}

	var result uploadResult
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.State != "COMPLETE" || result.Version != "1.2.0" || result.Build != "7" || result.BuildID != "B-7" {
		t.Errorf("result = %+v", result)
	}
	if len(result.Warnings) != 1 || !strings.Contains(h.errOut.String(), "SDK version issue") {
		t.Errorf("the warning was not reported: %+v\n%s", result.Warnings, h.errOut)
	}
}

func TestBuildUploadReportsWhyAppleRefused(t *testing.T) {
	t.Parallel()

	fake := &fakeConnect{uploadStates: []string{
		`{"state":"FAILED","errors":[{"code":"90062","description":"This bundle is invalid"}]}`,
	}}
	h, _ := uploadHarness(t, fake)
	if code := h.run("build", "upload", writeFixtureIPA(t, "1.2.0", "7")); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(h.errOut.String(), "90062: This bundle is invalid") {
		t.Errorf("stderr = %s", h.errOut)
	}
}

func TestBuildUploadRefusesAnotherAppsIPA(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{})
	if code := h.run("build", "upload", writeFixtureIPA(t, "1.2.0", "7"), "--bundle-id", "com.example.other"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(h.errOut.String(), "com.example.app, not com.example.other") {
		t.Errorf("stderr = %s", h.errOut)
	}
	if code := h.run("build", "upload", writeFixtureArchive(t, "1.2.0", "7")); code != 2 {
		t.Fatalf("exit %d for an archive, want 2", code)
	}
}
