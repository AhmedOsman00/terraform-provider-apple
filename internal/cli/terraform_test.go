// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
)

// teamOnlyConnect answers the two collections `--only team` reads, and
// counts every request so a refusal can be shown to have asked Apple nothing.
type teamOnlyConnect struct {
	requests int
}

func (f *teamOnlyConnect) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests++
	switch r.URL.Path {
	case "/v1/users":
		fmt.Fprint(w, `{"data":[{"type":"users","id":"U1","attributes":{"username":"dev@example.com",`+
			`"roles":["DEVELOPER"],"allAppsVisible":true}}],"links":{}}`)
	case "/v1/userInvitations":
		fmt.Fprint(w, `{"data":[],"links":{}}`)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newExportHarness(t *testing.T) (*harness, *teamOnlyConnect) {
	t.Helper()
	fake := &teamOnlyConnect{}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	h := &harness{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	h.app = &app{
		out:    h.out,
		errOut: h.errOut,
		newClient: func() (*apple.Client, error) {
			return &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}, nil
		},
		now:   time.Now,
		sleep: func(time.Duration) {},
	}
	return h, fake
}

func TestTerraformExportWritesConfigurationAndNotes(t *testing.T) {
	t.Parallel()
	h, _ := newExportHarness(t)
	dir := filepath.Join(t.TempDir(), "account")

	if code := h.run("terraform", "export", "--out", dir, "--only", "team"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}

	team, err := os.ReadFile(filepath.Join(dir, "team.tf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`to = apple_user.dev_example_com`, `id = "dev@example.com"`, `username         = "dev@example.com"`} {
		if !strings.Contains(string(team), want) {
			t.Errorf("team.tf lacks %q:\n%s", want, team)
		}
	}
	for _, name := range []string{"providers.tf", "EXPORT_NOTES.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}
	if !strings.Contains(h.out.String(), "1 records to import") || !strings.Contains(h.out.String(), "terraform plan") {
		t.Errorf("summary does not give the count and next step: %s", h.out)
	}
}

func TestTerraformExportReportsAsJSON(t *testing.T) {
	t.Parallel()
	h, _ := newExportHarness(t)

	if code := h.run("terraform", "export", "--json", "--out", t.TempDir(), "--only", "team"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errOut)
	}

	var summary struct {
		Files   []string       `json:"files"`
		Imports map[string]int `json:"imports"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &summary); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, h.out)
	}
	if summary.Imports["apple_user"] != 1 || len(summary.Files) == 0 {
		t.Errorf("summary = %+v", summary)
	}
}

func TestTerraformExportRefusesADirectoryInUse(t *testing.T) {
	t.Parallel()
	h, fake := newExportHarness(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := h.run("terraform", "export", "--out", dir); code != 2 {
		t.Fatalf("exit %d, want 2: %s", code, h.errOut)
	}
	if fake.requests != 0 {
		t.Errorf("asked Apple %d times before refusing", fake.requests)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "main.tf")); string(got) != "# mine\n" {
		t.Errorf("main.tf was changed: %q", got)
	}

	if code := h.run("terraform", "export", "--out", dir, "--only", "team", "--force"); code != 0 {
		t.Fatalf("--force: exit %d: %s", code, h.errOut)
	}
}

func TestTerraformExportRejectsAnUnknownPart(t *testing.T) {
	t.Parallel()
	h, _ := newExportHarness(t)

	if code := h.run("terraform", "export", "--out", t.TempDir(), "--only", "billing"); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(h.errOut.String(), "signing, app, products, testflight, team") {
		t.Errorf("error does not list the parts: %s", h.errOut)
	}
}
