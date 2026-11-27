// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// claimBundleIDs decides which app record owns each Bundle ID: the app whose
// own bundle identifier is the identifier itself, or the longest one it sits
// under. com.acme.app.widget goes with com.acme.app, while com.acme.app.staging
// goes with the staging app when one exists, and its own extensions with it.
// apps must be every app on the team, not only the ones being exported, or an
// app left out would have its Bundle ID claimed by the app above it.
//
// The result maps a Bundle ID record's ID to the owning app's ID. A Bundle ID
// no app claims is absent. Apple treats bundle identifiers as
// case-insensitive, so the match is too.
func claimBundleIDs(apps []models.App, bundleIDs []models.BundleID) map[string]string {
	owners := map[string]string{}
	for _, bundleID := range bundleIDs {
		identifier := strings.ToLower(bundleID.Attributes.Identifier)
		best := ""
		for _, app := range apps {
			own := strings.ToLower(app.Attributes.BundleID)
			if own == "" || len(own) <= len(best) {
				continue
			}
			if identifier == own || strings.HasPrefix(identifier, own+".") {
				best = own
				owners[bundleID.ID] = app.ID
			}
		}
	}
	return owners
}

// relatedSeparators are what a Bundle ID that was meant to sit under an app
// most often uses instead of a dot.
var relatedSeparators = strings.NewReplacer("-", ".", "_", ".")

// unclaimedBundleIDs lists the Bundle IDs that look as though they belong to
// one of the selected apps but sit outside its prefix (com-acme-app-widget
// beside com.acme.app), which the export reports rather than guesses at. A
// Bundle ID any app claims is never in it.
func unclaimedBundleIDs(selected []models.App, bundleIDs []models.BundleID, owners map[string]string) map[string][]models.BundleID {
	related := map[string][]models.BundleID{}
	for _, bundleID := range bundleIDs {
		if _, claimed := owners[bundleID.ID]; claimed {
			continue
		}
		identifier := relatedSeparators.Replace(strings.ToLower(bundleID.Attributes.Identifier))
		for _, app := range selected {
			own := relatedSeparators.Replace(strings.ToLower(app.Attributes.BundleID))
			if own != "" && (identifier == own || strings.HasPrefix(identifier, own+".")) {
				related[app.ID] = append(related[app.ID], bundleID)
				break
			}
		}
	}
	return related
}
