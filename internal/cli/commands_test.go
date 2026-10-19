// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
)

// fakeConnect is the slice of App Store Connect the commands read: one app,
// its versions, and builds whose processing state advances on every look.
type fakeConnect struct {
	t        *testing.T
	versions map[string]string
	locales  []string
	builds   map[string][]string
	states   []string

	// partsURL is where build upload parts go: a TLS server of the test's.
	partsURL     string
	uploadStates []string
	uploadLooks  int

	mu        sync.Mutex
	looks     int
	whatsNew  map[string]string
	requested []string
}

func (f *fakeConnect) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requested = append(f.requested, r.Method+" "+r.URL.Path)

	data := func(items []string) {
		fmt.Fprintf(w, `{"data":[%s],"links":{}}`, strings.Join(items, ","))
	}

	switch {
	case r.URL.Path == "/v1/apps":
		data([]string{`{"type":"apps","id":"APP1","attributes":{"bundleId":"com.example.app"}}`})

	case r.URL.Path == "/v1/apps/APP1/appStoreVersions":
		var items []string
		for version, state := range f.versions {
			items = append(items, fmt.Sprintf(`{"type":"appStoreVersions","id":"V-%s","attributes":{"versionString":%q,"appVersionState":%q}}`,
				version, version, state))
		}
		data(items)

	case strings.HasSuffix(r.URL.Path, "/appStoreVersionLocalizations"):
		var items []string
		for _, locale := range f.locales {
			items = append(items, fmt.Sprintf(`{"type":"appStoreVersionLocalizations","id":"L-%s","attributes":{"locale":%q}}`, locale, locale))
		}
		data(items)

	case r.URL.Path == "/v1/builds":
		train := r.URL.Query().Get("filter[preReleaseVersion.version]")
		number := r.URL.Query().Get("filter[version]")
		var items []string
		if number == "" {
			for _, build := range f.builds[train] {
				items = append(items, fmt.Sprintf(`{"type":"builds","id":"B-%s","attributes":{"version":%q}}`, build, build))
			}
		} else if f.looks < len(f.states) {
			if state := f.states[f.looks]; state != "" {
				items = append(items, fmt.Sprintf(`{"type":"builds","id":"B-%s","attributes":{"version":%q,"processingState":%q}}`,
					number, number, state))
			}
			f.looks++
		}
		data(items)

	case r.URL.Path == "/v1/builds/B-7/betaBuildLocalizations":
		data(nil)

	case r.Method == http.MethodPost && r.URL.Path == "/v1/betaBuildLocalizations":
		var body struct {
			Data struct {
				Attributes struct {
					Locale   string `json:"locale"`
					WhatsNew string `json:"whatsNew"`
				} `json:"attributes"`
			} `json:"data"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.whatsNew[body.Data.Attributes.Locale] = body.Data.Attributes.WhatsNew
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"data":{"type":"betaBuildLocalizations","id":"BL1","attributes":{}}}`)

	case r.Method == http.MethodPost && r.URL.Path == "/v1/buildUploads":
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"data":{"type":"buildUploads","id":"U1","attributes":{"state":{"state":"AWAITING_UPLOAD"}}}}`)

	case r.Method == http.MethodPost && r.URL.Path == "/v1/buildUploadFiles":
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"data":{"type":"buildUploadFiles","id":"F1","attributes":{"uploadOperations":[`+
			`{"method":"PUT","url":%q,"offset":0,"length":%d}]}}}`, f.partsURL, f.uploadSize(r))

	case r.Method == http.MethodPatch && r.URL.Path == "/v1/buildUploadFiles/F1":
		fmt.Fprint(w, `{"data":{"type":"buildUploadFiles","id":"F1","attributes":{}}}`)

	case r.Method == http.MethodGet && r.URL.Path == "/v1/buildUploads/U1":
		state := f.uploadStates[min(f.uploadLooks, len(f.uploadStates)-1)]
		f.uploadLooks++
		fmt.Fprintf(w, `{"data":{"type":"buildUploads","id":"U1","attributes":{"state":%s},`+
			`"relationships":{"build":{"data":{"type":"builds","id":"B-7"}}}}}`, state)

	default:
		f.t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotFound)
	}
}

// uploadSize is the fileSize a reservation asked for.
func (f *fakeConnect) uploadSize(r *http.Request) int64 {
	var body struct {
		Data struct {
			Attributes struct {
				FileSize int64 `json:"fileSize"`
			} `json:"attributes"`
		} `json:"data"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body.Data.Attributes.FileSize
}

type harness struct {
	app    *app
	out    *bytes.Buffer
	errOut *bytes.Buffer
	sleeps int
}

func newHarness(t *testing.T, fake *fakeConnect) *harness {
	t.Helper()
	fake.t = t
	if fake.whatsNew == nil {
		fake.whatsNew = map[string]string{}
	}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	h := &harness{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	clock := time.Date(2027, 2, 1, 12, 0, 0, 0, time.UTC)
	h.app = &app{
		out:    h.out,
		errOut: h.errOut,
		newClient: func() (*apple.Client, error) {
			return &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}, nil
		},
		now: func() time.Time { return clock },
		sleep: func(d time.Duration) {
			h.sleeps++
			clock = clock.Add(d)
		},
	}
	return h
}

func (h *harness) run(args ...string) int {
	return run(h.app, "test", args)
}

func TestVersionNextTakesTheMinorAfterTheNewestRelease(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{
		versions: map[string]string{"1.0": "REPLACED_WITH_NEW_VERSION", "1.1.0": "READY_FOR_DISTRIBUTION"},
		builds:   map[string][]string{"1.2.0": {"1", "4", "2"}},
	})
	if code := h.run("version", "next", "--bundle-id", "com.example.app"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if got := h.out.String(); got != "1.2.0 5\n" {
		t.Fatalf("stdout = %q, want \"1.2.0 5\\n\"", got)
	}
}

func TestVersionNextTakesAYearlyMajor(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{versions: map[string]string{"1.3.0": "READY_FOR_DISTRIBUTION"}})
	if code := h.run("version", "next", "--bundle-id", "com.example.app", "--yearly-major-from", "2026", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	var result nextResult
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Version != "2.0.0" || result.Build != "1" || result.NewestReleased != "1.3.0" {
		t.Fatalf("result = %+v; want 2.0.0 (1) after 1.3.0", result)
	}
}

func TestVersionNextRefusesAVersionNotAboveTheNewestRelease(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{versions: map[string]string{"1.1.0": "READY_FOR_DISTRIBUTION"}})
	if code := h.run("version", "next", "1.1", "--bundle-id", "com.example.app"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(h.errOut.String(), "not above the newest released version, 1.1.0") {
		t.Fatalf("stderr = %s", h.errOut)
	}
}

func TestVersionNextNeedsABundleID(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{})
	if code := h.run("version", "next"); code != 2 {
		t.Fatalf("exit %d, want 2 for a usage error", code)
	}
}

func TestFlagsReadTheirEnvironmentVariable(t *testing.T) {
	t.Setenv("APPLE_BUNDLE_ID", "com.example.app")

	h := newHarness(t, &fakeConnect{versions: map[string]string{"1.0.0": "READY_FOR_DISTRIBUTION"}})
	if code := h.run("version", "next"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if got := h.out.String(); got != "1.1.0 1\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func writeNotes(t *testing.T, version string, locales ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, locale := range locales {
		path := filepath.Join(dir, version, locale+".txt")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("Fixes.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReleasePreflightPassesWithNotesForEveryLocale(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{
		versions: map[string]string{"1.0.0": "READY_FOR_DISTRIBUTION"},
		locales:  []string{"ar-SA", "en-US"},
	})
	notes := writeNotes(t, "1.1.0", "en-US", "ar-SA")
	if code := h.run("release", "preflight", "1.1.0", "--bundle-id", "com.example.app", "--notes-dir", notes); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
}

func TestReleasePreflightReportsEveryProblem(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{
		versions: map[string]string{"1.0.0": "READY_FOR_DISTRIBUTION", "1.0.1": "WAITING_FOR_REVIEW"},
		locales:  []string{"ar-SA", "en-GB", "en-US"},
	})
	notes := writeNotes(t, "1.1.0", "en-US")
	if code := h.run("release", "preflight", "1.1.0", "--bundle-id", "com.example.app", "--notes-dir", notes); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	for _, want := range []string{"1.0.1 is WAITING_FOR_REVIEW", "1.1.0/ar-SA.txt", "1.1.0/en-GB.txt"} {
		if !strings.Contains(h.errOut.String(), want) {
			t.Errorf("stderr is missing %q:\n%s", want, h.errOut)
		}
	}
}

func TestReleasePreflightLetsLocalesBeNamed(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{
		versions: map[string]string{"1.0.0": "READY_FOR_DISTRIBUTION", "1.1.0": "PREPARE_FOR_SUBMISSION"},
		locales:  []string{"ar-SA", "en-GB", "en-US"},
	})
	notes := writeNotes(t, "1.2.0", "en-US", "ar-SA")
	code := h.run("release", "preflight", "1.2.0", "--bundle-id", "com.example.app", "--notes-dir", notes,
		"--locales", "en-US,ar-SA")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if !strings.Contains(h.errOut.String(), "Being prepared: 1.1.0") {
		t.Errorf("the version being prepared was not reported:\n%s", h.errOut)
	}
}

func TestReleasePreflightNeedsNoNotesForTheFirstVersion(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{versions: map[string]string{"1.0": "PREPARE_FOR_SUBMISSION"}})
	if code := h.run("release", "preflight", "1.0", "--bundle-id", "com.example.app"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
}

func TestBuildWaitPollsUntilValidAndSetsWhatToTest(t *testing.T) {
	t.Parallel()

	fake := &fakeConnect{states: []string{"", "PROCESSING", "VALID"}}
	h := newHarness(t, fake)
	notes := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notes, []byte("Try the new export."), 0o644); err != nil {
		t.Fatal(err)
	}

	code := h.run("build", "wait", "1.2.0", "7", "--bundle-id", "com.example.app", "--what-to-test", notes)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if h.sleeps != 2 {
		t.Errorf("slept %d times, want 2", h.sleeps)
	}
	if got := fake.whatsNew["en-US"]; got != "Try the new export." {
		t.Errorf("What to Test = %q", got)
	}
	if got := h.out.String(); got != "VALID\n" {
		t.Errorf("stdout = %q", got)
	}
}

func TestBuildWaitStopsOnAFailedBuild(t *testing.T) {
	t.Parallel()

	h := newHarness(t, &fakeConnect{states: []string{"PROCESSING", "INVALID"}})
	if code := h.run("build", "wait", "1.2.0", "7", "--bundle-id", "com.example.app"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(h.errOut.String(), "INVALID") {
		t.Fatalf("stderr = %s", h.errOut)
	}
}

func TestBuildWaitGivesUpAtTheTimeout(t *testing.T) {
	t.Parallel()

	states := make([]string, 10)
	h := newHarness(t, &fakeConnect{states: states})
	code := h.run("build", "wait", "1.2.0", "7", "--bundle-id", "com.example.app", "--timeout", "3m", "--interval", "1m")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if h.sleeps != 3 {
		t.Errorf("slept %d times, want 3", h.sleeps)
	}
}

// TestLongCommandsRenewTheirClient covers a wait outliving the provider's
// 20-minute token: a new client is built before the old one's token lapses.
func TestLongCommandsRenewTheirClient(t *testing.T) {
	t.Parallel()

	built := 0
	clock := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	a := &app{
		out: io.Discard, errOut: io.Discard,
		now:       func() time.Time { return clock },
		newClient: func() (*apple.Client, error) { built++; return &apple.Client{}, nil },
	}

	for range 3 {
		if _, err := a.api(); err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(10 * time.Minute)
	}
	if built != 2 {
		t.Fatalf("built %d clients over 30 minutes, want 2", built)
	}
}

func TestVersionSetAndCurrent(t *testing.T) {
	t.Parallel()

	project := filepath.Join(t.TempDir(), "App.xcodeproj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.pbxproj"), []byte(pbxprojFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	h := newHarness(t, &fakeConnect{})
	if code := h.run("version", "set", "2.0.0", "3", "--project", project); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	h.out.Reset()
	if code := h.run("version", "current", "--project", project); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}
	if got := h.out.String(); got != "2.0.0 3\n" {
		t.Fatalf("stdout = %q", got)
	}

	if code := h.run("version", "set", "2.0.0.1", "3", "--project", project); code != 2 {
		t.Fatalf("exit %d for a four-part version, want 2", code)
	}
}
