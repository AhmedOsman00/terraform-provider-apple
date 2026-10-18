// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// The CLI's acceptance tests only read: they ask App Store Connect about an
// existing app and build, and change nothing. APPLE_TEST_BUNDLE_ID names the
// app; the build tests reuse the provider suite's APPLE_TEST_BUILD_NUMBER and
// APPLE_TEST_BUILD_PRE_RELEASE_VERSION.
func newAcceptanceApp(t *testing.T) (*app, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests run with TF_ACC=1")
	}
	bundleID := os.Getenv("APPLE_TEST_BUNDLE_ID")
	if bundleID == "" {
		t.Skip("APPLE_TEST_BUNDLE_ID names the app the CLI acceptance tests read")
	}
	if _, err := clientFromEnvironment(); err != nil {
		t.Skip(err.Error())
	}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	return &app{
		out:       out,
		errOut:    errOut,
		newClient: clientFromEnvironment,
		now:       time.Now,
		sleep:     time.Sleep,
	}, out, errOut, bundleID
}

func TestAccCLIVersionNext(t *testing.T) {
	a, out, errOut, bundleID := newAcceptanceApp(t)

	if code := run(a, "test", []string{"version", "next", "--bundle-id", bundleID, "--json"}); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var result nextResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !versionPattern.MatchString(result.Version) || result.Build == "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestAccCLIReleasePreflight(t *testing.T) {
	a, _, errOut, bundleID := newAcceptanceApp(t)

	code := run(a, "test", []string{"release", "preflight", "999.0.0", "--bundle-id", bundleID,
		"--notes-dir", t.TempDir()})
	if code == 2 || !strings.Contains(errOut.String(), "Release 999.0.0") {
		t.Fatalf("exit %d: %s", code, errOut)
	}
}

func TestAccCLIBuildWaitExists(t *testing.T) {
	a, out, errOut, bundleID := newAcceptanceApp(t)
	build, train := os.Getenv("APPLE_TEST_BUILD_NUMBER"), os.Getenv("APPLE_TEST_BUILD_PRE_RELEASE_VERSION")
	if build == "" || train == "" {
		t.Skip("APPLE_TEST_BUILD_NUMBER and APPLE_TEST_BUILD_PRE_RELEASE_VERSION name an uploaded build")
	}

	code := run(a, "test", []string{"build", "wait", train, build, "--bundle-id", bundleID,
		"--until", "exists", "--timeout", "1m"})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatal("no processing state printed")
	}
}
