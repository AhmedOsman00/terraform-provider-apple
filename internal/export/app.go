// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
	"github.com/zclconf/go-cty/cty"
)

// selectApps resolves Options.Apps, or lists every app on the team.
func (e *exporter) selectApps() ([]models.App, error) {
	if len(e.opts.Apps) == 0 {
		e.progress("Reading apps")
		apps, err := e.src.GetApps()
		if err != nil {
			if err := e.failed("apps", err); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return apps, nil
	}

	apps := make([]models.App, 0, len(e.opts.Apps))
	for _, name := range e.opts.Apps {
		var (
			app *models.App
			err error
		)
		if strings.Contains(name, ".") {
			app, err = e.src.GetAppByBundleID(name)
		} else {
			app, err = e.src.GetApp(name)
		}
		if err != nil {
			if strings.Contains(err.Error(), "status 401") {
				return nil, fmt.Errorf("%w: reading app %s: %v", errFatal, name, err)
			}
			return nil, fmt.Errorf("could not find app %q (give an App Store Connect app ID or a bundle identifier): %w", name, err)
		}
		apps = append(apps, *app)
	}
	return apps, nil
}

// exportApp writes one app's listing, products and TestFlight setup, each in
// a file of its own named after the bundle identifier.
func (e *exporter) exportApp(app models.App) error {
	base := slug(app.Attributes.BundleID)
	if base == "unnamed" {
		base = slug(app.ID)
	}
	e.progress("Reading %s (%s)", app.Attributes.Name, app.Attributes.BundleID)

	if e.wants(DomainApp) {
		if err := e.exportListing(e.file(base+".tf"), app); err != nil {
			return err
		}
	}
	if e.wants(DomainProducts) {
		if err := e.exportProducts(e.file(base+"_products.tf"), app); err != nil {
			return err
		}
	}
	if e.wants(DomainTestFlight) {
		if err := e.exportTestFlight(e.file(base+"_testflight.tf"), app); err != nil {
			return err
		}
	}
	return nil
}

func (e *exporter) exportListing(f *file, app models.App) error {
	bundle := app.Attributes.BundleID
	f.comment(fmt.Sprintf("App Store listing for %s (%s, app ID %s).", app.Attributes.Name, bundle, app.ID))

	settings, _ := e.resource(f, "apple_app_settings", app.ID, app.ID, bundle)
	settings.str("app_id", app.ID)
	if app.Attributes.PrimaryLocale != "" {
		settings.str("primary_locale", app.Attributes.PrimaryLocale)
	}
	settings.optStr("content_rights_declaration", app.Attributes.ContentRightsDeclaration)
	settings.optBool("streamlined_purchasing_enabled", app.Attributes.StreamlinedPurchasingEnabled)
	settings.optStr("accessibility_url", app.Attributes.AccessibilityURL)
	settings.optStr("subscription_status_url", app.Attributes.SubscriptionStatusURL)
	settings.optStr("subscription_status_url_version", app.Attributes.SubscriptionStatusURLVersion)
	settings.optStr("subscription_status_url_for_sandbox", app.Attributes.SubscriptionStatusURLForSandbox)
	settings.optStr("subscription_status_url_version_for_sandbox", app.Attributes.SubscriptionStatusURLVersionForSandbox)

	if err := e.exportAppInfo(f, app); err != nil {
		return err
	}
	if err := e.exportVersions(f, app); err != nil {
		return err
	}
	if err := e.exportAppPrices(f, app); err != nil {
		return err
	}
	if err := e.exportAppAvailability(f, app); err != nil {
		return err
	}

	e.note(Skipped, "", "%s: App Privacy, app preview videos, the build upload, export compliance and submission have no resource; see remaining_manual_steps in examples/app-listing.", bundle)
	return nil
}

// exportAppInfo writes what hangs off the app info being prepared: the
// categories, the localized name and subtitle, and the age rating.
func (e *exporter) exportAppInfo(f *file, app models.App) error {
	bundle := app.Attributes.BundleID

	info, err := e.src.GetEditableAppInfo(app.ID)
	if err != nil {
		if isNotFound(err) || strings.Contains(err.Error(), "editable") || strings.Contains(err.Error(), "no app info") {
			e.note(Skipped, "", "%s: no app info is being prepared, so its categories, localized names and age rating were left out. They become editable, and exportable, once a new version is created.", bundle)
			return nil
		}
		return e.failed(bundle+"'s app info", err)
	}

	b, _ := e.resource(f, "apple_app_info", info.ID, app.ID, bundle)
	b.str("app_id", app.ID)
	if r := info.Relationships; r != nil {
		for _, category := range []struct {
			name string
			id   *models.ResourceIdentifier
		}{
			{"primary_category", r.PrimaryCategory},
			{"primary_subcategory_one", r.PrimarySubcategoryOne},
			{"primary_subcategory_two", r.PrimarySubcategoryTwo},
			{"secondary_category", r.SecondaryCategory},
			{"secondary_subcategory_one", r.SecondarySubcategoryOne},
			{"secondary_subcategory_two", r.SecondarySubcategoryTwo},
		} {
			if id := relationshipID(category.id); id != "" {
				b.str(category.name, id)
			}
		}
	}

	localizations, err := e.src.GetAppInfoLocalizations(info.ID)
	if err != nil {
		if err := e.failed(bundle+"'s localized names", err); err != nil {
			return err
		}
	}
	for _, localization := range localizations {
		locale := localization.Attributes.Locale
		l, _ := e.resource(f, "apple_app_info_localization", localization.ID, app.ID+"/"+locale, bundle, locale)
		l.str("app_id", app.ID)
		l.str("locale", locale)
		l.str("name", localization.Attributes.Name)
		l.optStr("subtitle", localization.Attributes.Subtitle)
		l.optStr("privacy_policy_url", localization.Attributes.PrivacyPolicyURL)
		l.optStr("privacy_choices_url", localization.Attributes.PrivacyChoicesURL)
		l.optStr("privacy_policy_text", localization.Attributes.PrivacyPolicyText)
	}

	declaration, err := e.src.GetAgeRatingDeclaration(info.ID)
	if err != nil {
		return e.failed(bundle+"'s age rating", err)
	}
	a, addr := e.resource(f, "apple_app_age_rating_declaration", declaration.ID, app.ID, bundle)
	a.str("app_id", app.ID)
	d := declaration.Attributes
	for _, answer := range []struct {
		name  string
		value *string
	}{
		{"alcohol_tobacco_or_drug_use_or_references", d.AlcoholTobaccoOrDrugUseOrReferences},
		{"contests", d.Contests},
		{"gambling_simulated", d.GamblingSimulated},
		{"guns_or_other_weapons", d.GunsOrOtherWeapons},
		{"horror_or_fear_themes", d.HorrorOrFearThemes},
		{"mature_or_suggestive_themes", d.MatureOrSuggestiveThemes},
		{"medical_or_treatment_information", d.MedicalOrTreatmentInformation},
		{"profanity_or_crude_humor", d.ProfanityOrCrudeHumor},
		{"sexual_content_graphic_and_nudity", d.SexualContentGraphicAndNudity},
		{"sexual_content_or_nudity", d.SexualContentOrNudity},
		{"violence_cartoon_or_fantasy", d.ViolenceCartoonOrFantasy},
		{"violence_realistic", d.ViolenceRealistic},
		{"violence_realistic_prolonged_graphic_or_sadistic", d.ViolenceRealisticProlongedGraphicOrSadistic},
		{"age_rating_override", d.AgeRatingOverride},
		{"age_rating_override_v2", d.AgeRatingOverrideV2},
		{"korea_age_rating_override", d.KoreaAgeRatingOverride},
		{"kids_age_band", d.KidsAgeBand},
		{"developer_age_rating_info_url", d.DeveloperAgeRatingInfoURL},
	} {
		a.optStr(answer.name, answer.value)
	}
	unanswered := 0
	for _, answer := range []struct {
		name  string
		value *bool
	}{
		{"advertising", d.Advertising},
		{"age_assurance", d.AgeAssurance},
		{"gambling", d.Gambling},
		{"health_or_wellness_topics", d.HealthOrWellnessTopics},
		{"loot_box", d.LootBox},
		{"messaging_and_chat", d.MessagingAndChat},
		{"parental_controls", d.ParentalControls},
		{"social_media", d.SocialMedia},
		{"social_media_age_restricted", d.SocialMediaAgeRestricted},
		{"unrestricted_web_access", d.UnrestrictedWebAccess},
		{"user_generated_content", d.UserGeneratedContent},
	} {
		if answer.value == nil {
			unanswered++
		}
		a.optBool(answer.name, answer.value)
	}
	if unanswered > 0 {
		e.note(Attention, addr.String(),
			"%d age rating questions are unanswered at Apple and are left out here. Leaving one out keeps Apple's answer rather than answering NONE, and an unanswered questionnaire blocks submission.", unanswered)
	}

	return nil
}

// editableVersionStates are the version states in which Apple accepts
// changes, matching the CLI's.
var editableVersionStates = map[string]bool{
	"PREPARE_FOR_SUBMISSION": true,
	"DEVELOPER_REJECTED":     true,
	"REJECTED":               true,
	"METADATA_REJECTED":      true,
	"INVALID_BINARY":         true,
}

// currentVersions picks one version per platform: the one being prepared if
// there is one, and otherwise the newest. An app holds only one editable
// version per platform, which is the one a configuration manages; the newest
// released one is where the next release is cut from.
func currentVersions(versions []models.AppStoreVersion) []models.AppStoreVersion {
	byPlatform := map[string]models.AppStoreVersion{}
	for _, version := range versions {
		platform := version.Attributes.Platform
		current, seen := byPlatform[platform]
		switch {
		case !seen:
			byPlatform[platform] = version
		case isEditable(version) && !isEditable(current):
			byPlatform[platform] = version
		case isEditable(version) == isEditable(current) &&
			compareVersions(version.Attributes.VersionString, current.Attributes.VersionString) > 0:
			byPlatform[platform] = version
		}
	}

	platforms := make([]string, 0, len(byPlatform))
	for platform := range byPlatform {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)

	picked := make([]models.AppStoreVersion, 0, len(platforms))
	for _, platform := range platforms {
		picked = append(picked, byPlatform[platform])
	}
	return picked
}

func isEditable(version models.AppStoreVersion) bool {
	return version.Attributes.AppVersionState != nil && editableVersionStates[*version.Attributes.AppVersionState]
}

func compareVersions(a, b string) int {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(x) || i < len(y); i++ {
		var m, n int
		if i < len(x) {
			m, _ = strconv.Atoi(x[i])
		}
		if i < len(y) {
			n, _ = strconv.Atoi(y[i])
		}
		if m != n {
			if m > n {
				return 1
			}
			return -1
		}
	}
	return 0
}

func (e *exporter) exportVersions(f *file, app models.App) error {
	bundle := app.Attributes.BundleID

	versions, err := e.src.GetAppStoreVersions(app.ID, "")
	if err != nil {
		return e.failed(bundle+"'s versions", err)
	}
	if len(versions) == 0 {
		e.note(Skipped, "", "%s has no App Store version yet, so none was exported. Add an apple_app_store_version to create the first one.", bundle)
		return nil
	}

	for _, version := range currentVersions(versions) {
		if err := e.exportVersion(f, app, version); err != nil {
			return err
		}
	}
	return nil
}

func (e *exporter) exportVersion(f *file, app models.App, version models.AppStoreVersion) error {
	bundle := app.Attributes.BundleID
	attributes := version.Attributes
	platform := attributes.Platform

	b, addr := e.resource(f, "apple_app_store_version", version.ID, version.ID, bundle, platform)
	b.str("app_id", app.ID)
	b.str("platform", platform)
	b.str("version_string", attributes.VersionString)
	b.optStr("copyright", attributes.Copyright)
	b.optStr("release_type", attributes.ReleaseType)
	b.optStr("review_type", attributes.ReviewType)
	b.optBool("uses_idfa", attributes.UsesIdfa)

	// The import adopts the attached build by number, as build_number. It
	// leaves pre_release_version null, which defaults to version_string.
	build, err := e.src.GetAppStoreVersionBuild(version.ID)
	if err != nil && !isNotFound(err) {
		if err := e.failed(fmt.Sprintf("the build attached to %s %s", bundle, attributes.VersionString), err); err != nil {
			return err
		}
	}
	if err == nil && build != nil && build.Attributes.Version != nil {
		b.str("build_number", *build.Attributes.Version)
	}

	if attributes.EarliestReleaseDate != nil {
		e.note(Attention, addr.String(),
			"Apple holds an earliest release date of %s, which an import does not record. Add earliest_release_date to keep it; leaving it out leaves Apple's value alone.", *attributes.EarliestReleaseDate)
	}
	if !isEditable(version) {
		state := "released"
		if attributes.AppVersionState != nil {
			state = *attributes.AppVersionState
		}
		e.note(Attention, addr.String(),
			"%s %s is %s, so Apple accepts no changes to it. Changing version_string replaces it with a new version, which is how the next release is cut.", bundle, attributes.VersionString, state)
	}

	localizations, err := e.src.GetAppStoreVersionLocalizations(version.ID)
	if err != nil {
		if err := e.failed(fmt.Sprintf("the localizations of %s %s", bundle, attributes.VersionString), err); err != nil {
			return err
		}
	}
	for _, localization := range localizations {
		locale := localization.Attributes.Locale
		l, _ := e.resource(f, "apple_app_store_version_localization", localization.ID,
			version.ID+"/"+locale, bundle, platform, locale)
		l.set("app_store_version_id", ref(addr.traversal("id")))
		l.str("locale", locale)
		l.optStr("description", localization.Attributes.Description)
		if keywords := splitKeywords(localization.Attributes.Keywords); keywords != nil {
			l.strings("keywords", keywords)
		}
		l.optStr("promotional_text", localization.Attributes.PromotionalText)
		l.optStr("marketing_url", localization.Attributes.MarketingURL)
		l.optStr("support_url", localization.Attributes.SupportURL)
		l.optStr("whats_new", localization.Attributes.WhatsNew)
	}
	if len(localizations) > 0 {
		e.note(Skipped, "", "%s %s (%s): screenshot sets were left out. Apple keeps a screenshot's bytes, not the file it came from, so an apple_app_screenshot_set needs the image files on disk; add one per display type pointing at them.", bundle, attributes.VersionString, platform)
	}

	detail, err := e.src.GetAppStoreReviewDetail(version.ID)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return e.failed(fmt.Sprintf("the App Review information of %s %s", bundle, attributes.VersionString), err)
	}
	r, raddr := e.resource(f, "apple_app_store_review_detail", detail.ID, version.ID, bundle, platform)
	r.set("app_store_version_id", ref(addr.traversal("id")))
	e.reviewContact(r, raddr, reviewDetail(detail.Attributes), "app_review_"+slug(bundle)+"_"+slug(platform)+"_demo_password")

	return nil
}

// contact is what both review detail resources share.
type contact struct {
	firstName, lastName, phone, email *string
	demoName, demoPassword            *string
	demoRequired                      *bool
	notes                             *string
}

func reviewDetail(a models.AppStoreReviewDetailAttributes) contact {
	return contact{a.ContactFirstName, a.ContactLastName, a.ContactPhone, a.ContactEmail,
		a.DemoAccountName, a.DemoAccountPassword, a.DemoAccountRequired, a.Notes}
}

// reviewContact writes the contact and demo account. The password goes
// through a variable: Apple masks or omits it, so the import cannot record
// it, and it does not belong in a file anyway.
func (e *exporter) reviewContact(b *block, addr address, c contact, passwordVariable string) {
	b.optStr("contact_first_name", c.firstName)
	b.optStr("contact_last_name", c.lastName)
	b.optStr("contact_phone", c.phone)
	b.optStr("contact_email", c.email)
	b.optStr("demo_account_name", c.demoName)
	b.optBool("demo_account_required", c.demoRequired)
	b.optStr("notes", c.notes)

	b.set("demo_account_password", e.secret(passwordVariable))
	if c.demoRequired != nil && *c.demoRequired {
		e.note(Attention, addr.String(),
			"A demo account is required, and Apple does not return its password. Set var.%s (it is sensitive, and still stored in state in plain text); until it is set, Terraform leaves the password Apple holds alone.", passwordVariable)
	}
}

// splitKeywords mirrors the provider's: Apple holds one comma-separated
// string, the configuration a list.
func splitKeywords(keywords *string) []string {
	if keywords == nil {
		return nil
	}
	var parts []string
	for _, part := range strings.Split(*keywords, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func (e *exporter) exportAppPrices(f *file, app models.App) error {
	bundle := app.Attributes.BundleID

	schedule, prices, err := e.src.GetAppPrices(app.ID)
	if err != nil {
		if isNotFound(err) {
			e.note(Skipped, "", "%s has no price schedule yet, so none was exported.", bundle)
			return nil
		}
		return e.failed(bundle+"'s price schedule", err)
	}

	base := ""
	if schedule.Relationships != nil {
		base = relationshipID(schedule.Relationships.BaseTerritory)
	}
	if base == "" {
		e.note(Skipped, "", "%s's price schedule reports no base territory, which the import requires, so it was left out.", bundle)
		return nil
	}

	b, addr := e.resource(f, "apple_app_price_schedule", schedule.ID, app.ID, bundle)
	b.str("app_id", app.ID)
	b.str("base_territory", base)

	// The import keys prices by price point, as the provider's reconcile does.
	byPoint := map[string]models.AppPrice{}
	for _, price := range prices {
		if price.Relationships == nil {
			continue
		}
		if point := relationshipID(price.Relationships.AppPricePoint); point != "" {
			byPoint[point] = price
		}
	}
	objects := make([]map[string]cty.Value, 0, len(byPoint))
	for _, point := range sortedKeys(byPoint) {
		price := byPoint[point]
		objects = append(objects, map[string]cty.Value{
			"price_point_id": cty.StringVal(point),
			"start_date":     optString(price.Attributes.StartDate),
			"end_date":       optString(price.Attributes.EndDate),
		})
	}
	b.objects("prices", objects)

	e.note(Attention, addr.String(),
		"This changes what %s costs. Apple publishes no DELETE for a price schedule, so a change applied here cannot be undone by removing it.", bundle)
	return nil
}

func (e *exporter) exportAppAvailability(f *file, app models.App) error {
	bundle := app.Attributes.BundleID

	availability, territories, err := e.src.GetAppAvailabilityWithTerritories(app.ID)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return e.failed(bundle+"'s availability", err)
	}

	b, _ := e.resource(f, "apple_app_availability", availability.ID, app.ID, bundle)
	b.str("app_id", app.ID)
	available := false
	if availability.Attributes.AvailableInNewTerritories != nil {
		available = *availability.Attributes.AvailableInNewTerritories
	}
	b.set("available_in_new_territories", literal(cty.BoolVal(available)))

	// The import keeps only the storefronts Apple reports as available.
	byCode := map[string]models.TerritoryAvailability{}
	for _, territory := range territories {
		if territory.Attributes.Available != nil && !*territory.Attributes.Available {
			continue
		}
		if territory.Relationships == nil {
			continue
		}
		if code := relationshipID(territory.Relationships.Territory); code != "" {
			byCode[code] = territory
		}
	}
	objects := make([]map[string]cty.Value, 0, len(byCode))
	for _, code := range sortedKeys(byCode) {
		territory := byCode[code]
		objects = append(objects, map[string]cty.Value{
			"territory":         cty.StringVal(code),
			"release_date":      optString(territory.Attributes.ReleaseDate),
			"pre_order_enabled": optBoolValue(territory.Attributes.PreOrderEnabled),
		})
	}
	b.objects("territories", objects)

	return nil
}

func relationshipID(r *models.ResourceIdentifier) string {
	if r == nil {
		return ""
	}
	return r.Data.ID
}
