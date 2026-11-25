// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// introducedIn is, per resource or data source the export writes, the first
// provider release in which it existed and imported the way its exporter
// writes it. MinProviderVersion must be the newest of them.
//
// A resource whose import had to be fixed for the empty-plan contract counts
// from the fix, not from the resource: apple_profile, apple_app_settings and
// apple_subscription imports were fixed alongside the export in 0.8.0, and
// apple_beta_group stopped replacing an imported external group there too.
// When an exporter starts relying on a newer release, raise its entry here
// and MinProviderVersion with it.
var introducedIn = map[string]string{
	"apple_bundle_id":                      "0.1.0",
	"apple_bundle_id_capability":           "0.1.0",
	"apple_certificate":                    "0.1.0",
	"apple_certificates":                   "0.1.0",
	"apple_device":                         "0.1.0",
	"apple_in_app_purchase":                "0.1.0",
	"apple_in_app_purchase_availability":   "0.1.0",
	"apple_in_app_purchase_localization":   "0.1.0",
	"apple_in_app_purchase_price_schedule": "0.1.0",
	"apple_merchant_id":                    "0.1.0",
	"apple_pass_type_id":                   "0.1.0",
	"apple_subscription_availability":      "0.1.0",
	"apple_subscription_group":             "0.1.0",
	"apple_subscription_localization":      "0.1.0",

	"apple_app_age_rating_declaration":      "0.2.0",
	"apple_app_availability":                "0.2.0",
	"apple_app_info":                        "0.2.0",
	"apple_app_info_localization":           "0.2.0",
	"apple_app_price_schedule":              "0.2.0",
	"apple_app_store_review_detail":         "0.2.0",
	"apple_app_store_version":               "0.2.0",
	"apple_app_store_version_localization":  "0.2.0",
	"apple_subscription_group_localization": "0.2.0",

	"apple_beta_app_localization":  "0.3.0",
	"apple_beta_app_review_detail": "0.3.0",

	"apple_beta_tester":                 "0.4.0",
	"apple_subscription_price_schedule": "0.4.0",
	"apple_user":                        "0.4.0",
	"apple_user_invitation":             "0.4.0",

	"apple_app_settings": "0.8.0",
	"apple_beta_group":   "0.8.0",
	"apple_profile":      "0.8.0",
	"apple_subscription": "0.8.0",
	"apple_subscription_introductory_offer_schedule": "0.8.0",
}

// TestMinProviderVersion fails when an exporter names a resource that
// introducedIn does not list, so adding one means deciding which release it
// needs, and when MinProviderVersion is not the newest entry.
func TestMinProviderVersion(t *testing.T) {
	written := exportedTypes(t)
	for _, name := range written {
		if _, ok := introducedIn[name]; !ok {
			t.Errorf("the export writes %s, which introducedIn does not list: add the first provider release it plans cleanly against", name)
		}
	}

	newest := "0.0.0"
	for _, name := range written {
		if v, ok := introducedIn[name]; ok && compareVersions(v, newest) > 0 {
			newest = v
		}
	}
	if MinProviderVersion != newest {
		t.Errorf("MinProviderVersion is %s, but the newest release an exported resource needs is %s", MinProviderVersion, newest)
	}
}

func TestProviderConstraint(t *testing.T) {
	major, _, _ := strings.Cut(MinProviderVersion, ".")
	next, err := strconv.Atoi(major)
	if err != nil {
		t.Fatalf("MinProviderVersion %q has no numeric major version", MinProviderVersion)
	}
	want := ">= " + MinProviderVersion + ", < " + strconv.Itoa(next+1) + ".0.0"
	if got := providerConstraint(); got != want {
		t.Errorf("providerConstraint() = %q, want %q", got, want)
	}
}

// exportedTypes is every apple_* type named in the package's non-test source,
// which is where each exporter names the block it writes.
func exportedTypes(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`"(apple_[a-z_]+)"`)
	seen := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range pattern.FindAllStringSubmatch(string(source), -1) {
			seen[match[1]] = true
		}
	}
	if len(seen) == 0 {
		t.Fatal("found no apple_* types in the export's source")
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
