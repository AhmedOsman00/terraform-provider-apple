// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

var update = flag.Bool("update", false, "rewrite the golden files under testdata/")

func runFake(t *testing.T, opts Options) (*Result, *fakeAccount) {
	t.Helper()
	account := &fakeAccount{}
	result, err := Run(account, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result, account
}

// TestGolden pins the whole output. Run with -update after an intended
// change, and read the diff: it is the configuration users will get.
func TestGolden(t *testing.T) {
	result, _ := runFake(t, Options{})

	dir := filepath.Join("testdata", "golden")
	if *update {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range result.Files {
			if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/export -update to create the golden files)", err)
	}
	if len(entries) != len(result.Files) {
		t.Errorf("golden has %d files, export wrote %d: %v", len(entries), len(result.Files), result.FileNames())
	}
	for name, content := range result.Files {
		want, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if string(want) != string(content) {
			t.Errorf("%s differs from testdata/golden/%s (run with -update and review the diff):\n%s", name, name, content)
		}
	}
}

func TestCertificatesAreNeverImported(t *testing.T) {
	result, _ := runFake(t, Options{Domains: []Domain{DomainSigning}})

	if n := result.Imports["apple_certificate"]; n != 0 {
		t.Fatalf("imported %d certificates; importing one reissues it and revokes the original", n)
	}
	signing := string(result.Files["signing.tf"])
	if !strings.Contains(signing, `data "apple_certificates" "example_distribution_distribution"`) {
		t.Errorf("no certificate data source in:\n%s", signing)
	}
	if !strings.Contains(signing, "data.apple_certificates.example_distribution_distribution.certificates[0].id") {
		t.Errorf("profile does not refer to the certificate data source:\n%s", signing)
	}
	// A certificate no longer listed is written as Apple's ID.
	if !strings.Contains(signing, `"CERT_GONE"`) {
		t.Errorf("revoked certificate ID is missing from the profile:\n%s", signing)
	}
}

func TestProfilesNameWhatTheyWereIssuedFor(t *testing.T) {
	result, _ := runFake(t, Options{Domains: []Domain{DomainSigning}})
	signing := string(result.Files["signing.tf"])

	for _, want := range []string{
		"bundle_id    = apple_bundle_id.com_example_app.id",
		"apple_device.ada_s_iphone.id",
	} {
		if !strings.Contains(signing, want) {
			t.Errorf("missing %q in:\n%s", want, signing)
		}
	}
	if result.Imports["apple_profile"] != 2 {
		t.Errorf("imported %d profiles, want the 2 active ones", result.Imports["apple_profile"])
	}
	if result.Imports["apple_device"] != 1 {
		t.Errorf("imported %d devices, want only the enabled one", result.Imports["apple_device"])
	}
}

func TestInterpolationIsEscaped(t *testing.T) {
	result, _ := runFake(t, Options{Domains: []Domain{DomainApp}})
	listing := string(result.Files["com_example_app.tf"])

	// Apple text is data. Written raw, ${done} would be a reference to
	// something called done, and the configuration would not load.
	if !strings.Contains(listing, `"Gets things $${done}"`) {
		t.Errorf("subtitle not escaped:\n%s", listing)
	}
}

func TestVersionsPickTheOneBeingPrepared(t *testing.T) {
	result, _ := runFake(t, Options{Domains: []Domain{DomainApp}})
	listing := string(result.Files["com_example_app.tf"])

	if !strings.Contains(listing, `id = "VER_NEW"`) || strings.Contains(listing, `"VER_OLD"`) {
		t.Errorf("expected the editable iOS version and not the released one:\n%s", listing)
	}
	if !strings.Contains(listing, `id = "VER_MAC"`) {
		t.Errorf("expected the newest macOS version when none is editable:\n%s", listing)
	}
	if !strings.Contains(listing, `build_number   = "42"`) {
		t.Errorf("attached build not exported:\n%s", listing)
	}
	if !strings.Contains(listing, `keywords             = ["tasks", "todo", "lists"]`) {
		t.Errorf("keywords not split:\n%s", listing)
	}
}

func TestSecretsBecomeNullVariables(t *testing.T) {
	result, _ := runFake(t, Options{})

	variables := string(result.Files["variables.tf"])
	for _, name := range []string{"app_review_com_example_app_ios_demo_password", "beta_review_com_example_app_demo_password"} {
		if !strings.Contains(variables, `variable "`+name+`"`) {
			t.Errorf("no variable %s in:\n%s", name, variables)
		}
	}
	if !strings.Contains(variables, "default     = null") || !strings.Contains(variables, "sensitive   = true") {
		t.Errorf("secret variables must be sensitive and default to null:\n%s", variables)
	}
	for name, content := range result.Files {
		if strings.Contains(string(content), "••••") {
			t.Errorf("%s contains the masked password Apple returned", name)
		}
	}
}

func TestTestFlightLeavesOutWhatWouldChurn(t *testing.T) {
	result, _ := runFake(t, Options{Domains: []Domain{DomainTestFlight}})
	testflight := string(result.Files["com_example_app_testflight.tf"])

	if strings.Contains(testflight, "someone@example.org") {
		t.Errorf("public link tester exported:\n%s", testflight)
	}
	if strings.Contains(testflight, "first_name") {
		t.Errorf("a tester's name forces replacement and the import does not record it:\n%s", testflight)
	}
	// The internal group has public_link_enabled = false at Apple, which the
	// provider rejects on an internal group.
	internal := testflight[strings.Index(testflight, `"com_example_app_team"`):strings.Index(testflight, `"com_example_app_public_beta"`)]
	if strings.Contains(internal, "public_link") {
		t.Errorf("public link attributes on an internal group:\n%s", internal)
	}
}

func TestTeamLeavesOutTheAccountHolder(t *testing.T) {
	result, _ := runFake(t, Options{Domains: []Domain{DomainTeam}})
	team := string(result.Files["team.tf"])

	if strings.Contains(team, "owner@example.com") {
		t.Errorf("account holder exported:\n%s", team)
	}
	if !strings.Contains(team, `roles                = ["APP_MANAGER", "DEVELOPER"]`) {
		t.Errorf("roles not written sorted:\n%s", team)
	}
	if !strings.Contains(team, `visible_apps         = ["6478123456"]`) {
		t.Errorf("visible apps missing for a restricted member:\n%s", team)
	}
}

func TestInAppPurchaseLocalizationsComeFromTheNewestVersion(t *testing.T) {
	_, account := runFake(t, Options{Domains: []Domain{DomainProducts}})

	if len(account.calls) != 1 || account.calls[0] != "GetInAppPurchaseLocalizations IAPV2" {
		t.Errorf("read localizations through %v, want only the newest version IAPV2", account.calls)
	}
}

func TestNotesSayWhatThePlanShouldBe(t *testing.T) {
	result, _ := runFake(t, Options{})
	notes := string(result.Files[NotesFile])

	if !strings.Contains(notes, "0 to add, 0 to change, 0 to destroy") {
		t.Errorf("notes do not state the expected plan:\n%s", notes)
	}
	for _, want := range []string{"screenshot", "Account Holder", "earliest release date", "public link"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes do not mention %q:\n%s", want, notes)
		}
	}
}

func TestLabelsAreUniqueAndValid(t *testing.T) {
	var l labels
	got := []string{
		l.next("apple_device", "Ada's iPhone"),
		l.next("apple_device", "Ada’s iPhone"),
		l.next("apple_device", "2nd phone"),
		l.next("apple_device", "!!!"),
		l.next("apple_profile", "Ada's iPhone"),
	}
	want := []string{"ada_s_iphone", "ada_s_iphone_2", "r_2nd_phone", "unnamed", "ada_s_iphone"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// unauthorized is an account whose credentials Apple rejects.
type unauthorized struct{ fakeAccount }

func (*unauthorized) GetBundleIDs() ([]models.BundleID, error) {
	return nil, errors.New("authentication failed (status 401): App Store Connect rejected the API credentials")
}

func TestCredentialFailureStopsTheExport(t *testing.T) {
	_, err := Run(&unauthorized{}, Options{Domains: []Domain{DomainSigning}})
	if !errors.Is(err, errFatal) {
		t.Fatalf("Run = %v, want a fatal error rather than a note per record", err)
	}
}
