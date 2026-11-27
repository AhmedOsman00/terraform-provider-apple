// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

const stagingAppID = "6478999999"

// scopedAccount is the fake account with a second app in it: an app with a
// widget extension and a lookalike Bundle ID outside its prefix, a staging app
// whose Bundle ID sits under the first one's, and a certificate both apps'
// profiles use.
type scopedAccount struct{ fakeAccount }

func (f *scopedAccount) staging() models.App {
	return models.App{ID: stagingAppID, Attributes: models.AppAttributes{
		Name: "Example Staging", BundleID: "com.example.app.staging", SKU: "EXAMPLE_STAGING", PrimaryLocale: "en-US",
	}}
}

func (f *scopedAccount) GetApps() ([]models.App, error) {
	return []models.App{f.app(), f.staging()}, nil
}

func (f *scopedAccount) GetApp(id string) (*models.App, error) {
	if id == stagingAppID {
		a := f.staging()
		return &a, nil
	}
	return f.fakeAccount.GetApp(id)
}

func (f *scopedAccount) GetAppByBundleID(bundleID string) (*models.App, error) {
	if bundleID == "com.example.app.staging" {
		a := f.staging()
		return &a, nil
	}
	return f.fakeAccount.GetAppByBundleID(bundleID)
}

func (f *scopedAccount) GetBundleIDs() ([]models.BundleID, error) {
	bundle := func(id, identifier, name string) models.BundleID {
		return models.BundleID{ID: id, Attributes: models.BundleIDAttributes{
			Identifier: identifier, Name: name, Platform: models.IOS, SeedID: "TEAM123456",
		}}
	}
	return []models.BundleID{
		bundle(bundleRef, "com.example.app", "Example App"),
		bundle("BWIDGET", "com.example.app.widget", "Example Widget"),
		bundle("BSTAGING", "com.example.app.staging", "Example Staging"),
		bundle("BDASHED", "com-example-app-tests", "Example Tests"),
		bundle("BOTHER", "org.example.unrelated", "Unrelated"),
	}, nil
}

func (f *scopedAccount) GetBundleIDCapabilities(string) ([]models.BundleIDCapability, error) {
	return nil, nil
}

func (f *scopedAccount) GetDevices() ([]models.Device, error) {
	devices, _ := f.fakeAccount.GetDevices()
	return append(devices, models.Device{ID: "DEV3", Attributes: models.DeviceAttributes{
		Name: "Spare Mac", UDID: "00006000-001A2B3C4D5E", Platform: models.DeviceMAC, Status: models.DeviceStatusENABLED,
	}}), nil
}

func (f *scopedAccount) GetCertificates() ([]models.Certificate, error) {
	certificates, _ := f.fakeAccount.GetCertificates()
	return append(certificates,
		models.Certificate{ID: "CERT2", Attributes: models.CertificateAttributes{
			SerialNumber: "7B2C3D4E5F60", DisplayName: "Staging Development", CertificateType: models.Development,
		}},
		models.Certificate{ID: "CERT3", Attributes: models.CertificateAttributes{
			SerialNumber: "8C3D4E5F6071", DisplayName: "Unused Development", CertificateType: models.Development,
		}},
	), nil
}

func (f *scopedAccount) GetProfiles() ([]models.Profile, error) {
	profile := func(id, name string, kind models.ProfileType) models.Profile {
		return models.Profile{ID: id, Attributes: models.ProfileAttributes{Name: name, ProfileType: ptr(kind), ProfileState: ptr(models.ProfileStateActive)}}
	}
	return []models.Profile{
		profile("PROF1", "Example App Store", models.ProfileTypeAppStore),
		profile("PROFW", "Example Widget Ad Hoc", models.ProfileTypeAdhoc),
		profile("PROFS", "Example Staging Development", models.ProfileTypeDevelopment),
		profile("PROFO", "Unrelated App Store", models.ProfileTypeAppStore),
	}, nil
}

func (f *scopedAccount) GetProfileLinkages(profileID string) (*apple.ProfileLinkages, error) {
	f.calls = append(f.calls, "GetProfileLinkages "+profileID)
	switch profileID {
	case "PROF1":
		return &apple.ProfileLinkages{BundleID: bundleRef, CertificateIDs: []string{"CERT1"}}, nil
	case "PROFW":
		return &apple.ProfileLinkages{BundleID: "BWIDGET", CertificateIDs: []string{"CERT1"}, DeviceIDs: []string{"DEV1", "DEV2"}}, nil
	case "PROFS":
		return &apple.ProfileLinkages{BundleID: "BSTAGING", CertificateIDs: []string{"CERT1", "CERT2"}, DeviceIDs: []string{"DEV1"}}, nil
	default:
		return &apple.ProfileLinkages{BundleID: "BOTHER", CertificateIDs: []string{"CERT3"}}, nil
	}
}

func runScoped(t *testing.T, opts Options) *Result {
	t.Helper()
	result, err := Run(&scopedAccount{}, opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result
}

// TestGoldenApp pins the output of --app com.example.app over the account
// with a staging app, the same way TestGolden pins a whole-account export.
func TestGoldenApp(t *testing.T) {
	result := runScoped(t, Options{Apps: []string{"com.example.app"}})
	compareGolden(t, filepath.Join("testdata", "golden_app"), result)
}

func TestAppExportsOnlyItsOwnSigning(t *testing.T) {
	result := runScoped(t, Options{Apps: []string{"com.example.app"}})

	if _, ok := result.Files["signing.tf"]; ok {
		t.Errorf("one app's export wrote signing.tf, which holds what several apps or the account share:\n%s", result.Files["signing.tf"])
	}
	signing := string(result.Files["com_example_app_signing.tf"])
	for _, want := range []string{
		`identifier = "com.example.app"`,
		`identifier = "com.example.app.widget"`,
		`"Example App Store"`,
		`"Example Widget Ad Hoc"`,
		`data "apple_certificates" "example_distribution_distribution"`,
		`udid_pattern = "^00008030-000A4D8E0AB8802E$"`,
		"data.apple_devices.ada_s_iphone.devices[0].id",
	} {
		if !strings.Contains(signing, want) {
			t.Errorf("missing %q in:\n%s", want, signing)
		}
	}
	for _, unwanted := range []string{
		"com.example.app.staging", // another app record's own Bundle ID
		"com-example-app-tests",   // outside the prefix: noted, not guessed at
		"org.example.unrelated",
		"Staging Development", "Unused Development", // certificates no profile of this app uses
		"Spare Mac",               // a device no profile of this app uses
		`resource "apple_device"`, // devices are the account's, so only read
		`resource "apple_merchant_id"`,
	} {
		for name, content := range result.Files {
			if strings.Contains(string(content), unwanted) && name != NotesFile {
				t.Errorf("%s contains %q", name, unwanted)
			}
		}
	}
	if _, ok := result.Files["team.tf"]; ok {
		t.Error("--app alone exported the team")
	}

	notes := string(result.Files[NotesFile])
	if !strings.Contains(notes, "com-example-app-tests looks as though it belongs to com.example.app") {
		t.Errorf("the lookalike Bundle ID is not reported as unclaimed:\n%s", notes)
	}
}

func TestSharedCertificateIsWrittenOnce(t *testing.T) {
	result := runScoped(t, Options{Apps: []string{"com.example.app", "com.example.app.staging"}, Parts: mustParts(t, "signing")})

	shared := string(result.Files["signing.tf"])
	if strings.Count(shared, `data "apple_certificates"`) != 1 || !strings.Contains(shared, `"6A1B2C3D4E5F"`) {
		t.Errorf("the certificate both apps use should be in signing.tf once:\n%s", shared)
	}
	staging := string(result.Files["com_example_app_staging_signing.tf"])
	if !strings.Contains(staging, `identifier = "com.example.app.staging"`) || !strings.Contains(staging, `"7B2C3D4E5F60"`) {
		t.Errorf("staging's Bundle ID and its own certificate belong in its file:\n%s", staging)
	}
	total := 0
	for _, content := range result.Files {
		total += strings.Count(string(content), `data "apple_certificates"`)
	}
	if total != 2 {
		t.Errorf("wrote %d certificate data sources, want 2 (shared and staging's own)", total)
	}
	// --only signing names the devices part, so it is exported as the
	// account's: every enabled device, as a resource, in signing.tf.
	if !strings.Contains(shared, `resource "apple_device" "spare_mac"`) {
		t.Errorf("--only signing did not export the devices as resources:\n%s", shared)
	}
	if !strings.Contains(shared, `resource "apple_merchant_id"`) {
		t.Errorf("--only signing did not export the merchant IDs:\n%s", shared)
	}
}

func TestExplicitOnlyWinsOverApp(t *testing.T) {
	result := runScoped(t, Options{Apps: []string{"com.example.app"}, Parts: mustParts(t, "users")})

	for name := range result.Files {
		switch name {
		case "team.tf", "providers.tf", NotesFile:
		default:
			t.Errorf("--app with --only users wrote %s", name)
		}
	}
	if result.Imports["apple_user_invitation"] != 0 {
		t.Error("--only users exported invitations")
	}
}

func TestContentsListsEveryBlock(t *testing.T) {
	result := runScoped(t, Options{Apps: []string{"com.example.app"}})

	imports := 0
	for _, entry := range result.Contents {
		if entry.Part == "" {
			t.Errorf("%s has no part", entry.Address)
		}
		if entry.Import {
			imports++
		}
		if strings.HasPrefix(entry.File, "com_example_app") && entry.App != "com.example.app" {
			t.Errorf("%s in %s is not attributed to the app", entry.Address, entry.File)
		}
	}
	if imports != result.ImportCount() {
		t.Errorf("contents list %d imports, the result counts %d", imports, result.ImportCount())
	}
}

// TestEveryTypeHasAPart keeps Parts, which --help prints, in step with what
// the exporters write.
func TestEveryTypeHasAPart(t *testing.T) {
	for _, name := range exportedTypes(t) {
		if partOf(name) == "" && partOf("data."+name) == "" {
			t.Errorf("the export writes %s, which no entry in Parts lists", name)
		}
	}
}

func compareGolden(t *testing.T, dir string, result *Result) {
	t.Helper()
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
		t.Errorf("%s has %d files, export wrote %d: %v", dir, len(entries), len(result.Files), result.FileNames())
	}
	for name, content := range result.Files {
		want, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if string(want) != string(content) {
			t.Errorf("%s differs from %s/%s (run with -update and review the diff):\n%s", name, dir, name, content)
		}
	}
}
