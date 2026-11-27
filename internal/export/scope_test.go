// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"reflect"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

func testApp(id, bundleID string) models.App {
	return models.App{ID: id, Attributes: models.AppAttributes{BundleID: bundleID}}
}

func testBundleID(id, identifier string) models.BundleID {
	return models.BundleID{ID: id, Attributes: models.BundleIDAttributes{Identifier: identifier}}
}

func TestClaimBundleIDs(t *testing.T) {
	apps := []models.App{
		testApp("APP", "com.acme.app"),
		testApp("STAGING", "com.acme.app.staging"),
		testApp("OTHER", "com.acme.other"),
		testApp("NOBUNDLE", ""),
	}
	bundleIDs := []models.BundleID{
		testBundleID("B_APP", "com.acme.app"),
		testBundleID("B_WIDGET", "com.acme.app.widget"),
		testBundleID("B_DEEP", "com.acme.app.widget.intents"),
		testBundleID("B_STAGING", "com.acme.app.staging"),
		testBundleID("B_STAGING_WIDGET", "com.acme.app.staging.widget"),
		testBundleID("B_CASE", "COM.Acme.App.Clip"),
		testBundleID("B_PREFIX_ONLY", "com.acme.application"),
		testBundleID("B_DASHED", "com-acme-app-widget"),
		testBundleID("B_WILDCARD", "com.acme.*"),
		testBundleID("B_OTHER", "com.acme.other"),
	}

	got := claimBundleIDs(apps, bundleIDs)
	want := map[string]string{
		"B_APP":            "APP",
		"B_WIDGET":         "APP",
		"B_DEEP":           "APP",
		"B_CASE":           "APP",
		"B_STAGING":        "STAGING",
		"B_STAGING_WIDGET": "STAGING",
		"B_OTHER":          "OTHER",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("claimBundleIDs =\n%v\nwant\n%v", got, want)
	}

	// The order apps come back in must not matter: the longest prefix wins.
	reversed := []models.App{apps[3], apps[2], apps[1], apps[0]}
	if got := claimBundleIDs(reversed, bundleIDs); !reflect.DeepEqual(got, want) {
		t.Errorf("with apps reversed, claimBundleIDs =\n%v\nwant\n%v", got, want)
	}
}

func TestUnclaimedBundleIDs(t *testing.T) {
	apps := []models.App{testApp("APP", "com.acme.app"), testApp("OTHER", "com.acme.other")}
	bundleIDs := []models.BundleID{
		testBundleID("B_APP", "com.acme.app"),
		testBundleID("B_DASHED", "com-acme-app-widget"),
		testBundleID("B_UNDERSCORE", "com.acme.app_tests"),
		testBundleID("B_PREFIX_ONLY", "com.acme.application"),
		testBundleID("B_OTHER_DASHED", "com-acme-other"),
		testBundleID("B_UNRELATED", "org.example.thing"),
	}
	owners := claimBundleIDs(apps, bundleIDs)

	// Only APP is being exported, so OTHER's lookalike is not reported.
	got := unclaimedBundleIDs(apps[:1], bundleIDs, owners)
	var ids []string
	for _, b := range got["APP"] {
		ids = append(ids, b.ID)
	}
	if want := []string{"B_DASHED", "B_UNDERSCORE"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("unclaimed for APP = %v, want %v", ids, want)
	}
	if len(got) != 1 {
		t.Errorf("unclaimed reported for apps not being exported: %v", got)
	}
}

func TestParsePart(t *testing.T) {
	for name, want := range map[string][]Part{
		"listing":     {PartListing},
		" Devices ":   {PartDevices},
		"app":         {PartListing},
		"team":        {PartUsers, PartInvitations},
		"signing":     {PartBundleIDs, PartProfiles, PartCertificates, PartDevices, PartMerchantIDs, PartPassTypeIDs},
		"invitations": {PartInvitations},
	} {
		got, err := ParsePart(name)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("ParsePart(%q) = %v, %v; want %v", name, got, err, want)
		}
	}
	if _, err := ParsePart("billing"); err == nil {
		t.Error("ParsePart(billing) did not fail")
	}
}
