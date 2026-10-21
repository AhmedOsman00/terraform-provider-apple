// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"fmt"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// exportTestFlight writes an app's beta groups and who is in them, its
// TestFlight page text and what the beta reviewers are told.
func (e *exporter) exportTestFlight(f *file, app models.App) error {
	bundle := app.Attributes.BundleID
	f.comment(fmt.Sprintf("TestFlight for %s (app ID %s).\nRemoving an apple_beta_tester takes that person out of the group.", app.Attributes.Name, app.ID))

	groups, err := e.src.GetBetaGroups(app.ID)
	if err != nil {
		if err := e.failed(bundle+"'s beta groups", err); err != nil {
			return err
		}
	}
	for _, group := range groups {
		if err := e.exportBetaGroup(f, app, group); err != nil {
			return err
		}
	}
	if len(groups) > 0 {
		e.note(Skipped, "", "%s: builds given to beta groups (apple_beta_group_build) and per-build \"What to Test\" notes (apple_beta_build_localization) were left out. Both belong to one build and change with every upload, so they are better written next to the pipeline that produces the build.", bundle)
	}

	localizations, err := e.src.GetBetaAppLocalizations(app.ID)
	if err != nil {
		if err := e.failed(bundle+"'s TestFlight page text", err); err != nil {
			return err
		}
	}
	for _, localization := range localizations {
		attributes := localization.Attributes
		b, _ := e.resource(f, "apple_beta_app_localization", localization.ID, localization.ID, bundle, attributes.Locale)
		b.str("app_id", app.ID)
		b.str("locale", attributes.Locale)
		b.optStr("description", attributes.Description)
		b.optStr("feedback_email", attributes.FeedbackEmail)
		b.optStr("marketing_url", attributes.MarketingURL)
		b.optStr("privacy_policy_url", attributes.PrivacyPolicyURL)
		b.optStr("tvos_privacy_policy", attributes.TVOSPrivacyPolicy)
	}

	detail, err := e.src.GetBetaAppReviewDetail(app.ID)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return e.failed(bundle+"'s beta review information", err)
	}
	b, addr := e.resource(f, "apple_beta_app_review_detail", detail.ID, app.ID, bundle)
	b.str("app_id", app.ID)
	a := detail.Attributes
	e.reviewContact(b, addr, contact{a.ContactFirstName, a.ContactLastName, a.ContactPhone, a.ContactEmail,
		a.DemoAccountName, a.DemoAccountPassword, a.DemoAccountRequired, a.Notes},
		"beta_review_"+slug(bundle)+"_demo_password")

	return nil
}

func (e *exporter) exportBetaGroup(f *file, app models.App, group models.BetaGroup) error {
	attributes := group.Attributes
	if attributes.Name == nil {
		e.note(Skipped, "", "A beta group of %s (%s) reports no name, so it was left out.", app.Attributes.BundleID, group.ID)
		return nil
	}
	name := *attributes.Name

	b, addr := e.resource(f, "apple_beta_group", group.ID, group.ID, app.Attributes.BundleID, name)
	b.str("app_id", app.ID)
	b.str("name", name)
	b.optBool("is_internal_group", attributes.IsInternalGroup)
	b.optBool("has_access_to_all_builds", attributes.HasAccessToAllBuilds)
	b.optBool("feedback_enabled", attributes.FeedbackEnabled)
	b.optBool("ios_builds_available_for_apple_silicon_mac", attributes.IOSBuildsAvailableForAppleSiliconMac)
	b.optBool("ios_builds_available_for_apple_vision", attributes.IOSBuildsAvailableForAppleVision)

	// The provider refuses public link attributes on an internal group, even
	// false ones, and a limit without its flag.
	internal := attributes.IsInternalGroup != nil && *attributes.IsInternalGroup
	if !internal {
		b.optBool("public_link_enabled", attributes.PublicLinkEnabled)
		if attributes.PublicLinkLimitEnabled != nil {
			b.optBool("public_link_limit_enabled", attributes.PublicLinkLimitEnabled)
			b.optInt("public_link_limit", attributes.PublicLinkLimit)
		}
	}

	testers, err := e.src.GetBetaGroupTesters(group.ID)
	if err != nil {
		return e.failed(fmt.Sprintf("the testers in beta group %q", name), err)
	}

	joinedByLink := 0
	for _, tester := range testers {
		email := ""
		if tester.Attributes.Email != nil {
			email = *tester.Attributes.Email
		}
		// Somebody who joined through the public link was never invited by
		// anyone, and naming them here would make the configuration the
		// list of who happened to click it.
		if tester.Attributes.InviteType != nil && *tester.Attributes.InviteType == "PUBLIC_LINK" {
			joinedByLink++
			continue
		}
		if email == "" {
			continue
		}

		// first_name and last_name are left out on purpose: the import does
		// not record them, both force replacement, and replacing a
		// membership takes the person out and invites them again.
		t, _ := e.resource(f, "apple_beta_tester", "", group.ID+"/"+email, name, email)
		t.set("group_id", ref(addr.traversal("id")))
		t.str("email", email)
	}
	if joinedByLink > 0 {
		e.note(Skipped, addr.String(), "%d testers who joined %q through its public link were left out.", joinedByLink, name)
	}

	return nil
}
