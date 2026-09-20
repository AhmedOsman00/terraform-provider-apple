// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"regexp"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// appSettingsModel maps the account-level settings on an app record.
type appSettingsModel struct {
	AppID                                  types.String `tfsdk:"app_id"`
	ContentRightsDeclaration               types.String `tfsdk:"content_rights_declaration"`
	PrimaryLocale                          types.String `tfsdk:"primary_locale"`
	AccessibilityURL                       types.String `tfsdk:"accessibility_url"`
	SubscriptionStatusURL                  types.String `tfsdk:"subscription_status_url"`
	SubscriptionStatusURLVersion           types.String `tfsdk:"subscription_status_url_version"`
	SubscriptionStatusURLForSandbox        types.String `tfsdk:"subscription_status_url_for_sandbox"`
	SubscriptionStatusURLVersionForSandbox types.String `tfsdk:"subscription_status_url_version_for_sandbox"`
	StreamlinedPurchasingEnabled           types.Bool   `tfsdk:"streamlined_purchasing_enabled"`
	Name                                   types.String `tfsdk:"name"`
	BundleID                               types.String `tfsdk:"bundle_id"`
	SKU                                    types.String `tfsdk:"sku"`
}

// appInfoModel maps the categories on an app's editable AppInfo.
type appInfoModel struct {
	ID                      types.String `tfsdk:"id"`
	AppID                   types.String `tfsdk:"app_id"`
	PrimaryCategory         types.String `tfsdk:"primary_category"`
	PrimarySubcategoryOne   types.String `tfsdk:"primary_subcategory_one"`
	PrimarySubcategoryTwo   types.String `tfsdk:"primary_subcategory_two"`
	SecondaryCategory       types.String `tfsdk:"secondary_category"`
	SecondarySubcategoryOne types.String `tfsdk:"secondary_subcategory_one"`
	SecondarySubcategoryTwo types.String `tfsdk:"secondary_subcategory_two"`
	State                   types.String `tfsdk:"state"`
	AppStoreAgeRating       types.String `tfsdk:"app_store_age_rating"`
}

// appInfoLocalizationModel maps the app's localized name and subtitle.
type appInfoLocalizationModel struct {
	ID                types.String `tfsdk:"id"`
	AppID             types.String `tfsdk:"app_id"`
	AppInfoID         types.String `tfsdk:"app_info_id"`
	Locale            types.String `tfsdk:"locale"`
	Name              types.String `tfsdk:"name"`
	Subtitle          types.String `tfsdk:"subtitle"`
	PrivacyPolicyURL  types.String `tfsdk:"privacy_policy_url"`
	PrivacyChoicesURL types.String `tfsdk:"privacy_choices_url"`
	PrivacyPolicyText types.String `tfsdk:"privacy_policy_text"`
}

// appStoreVersionModel maps one release of an app.
type appStoreVersionModel struct {
	ID                  types.String `tfsdk:"id"`
	AppID               types.String `tfsdk:"app_id"`
	Platform            types.String `tfsdk:"platform"`
	VersionString       types.String `tfsdk:"version_string"`
	Copyright           types.String `tfsdk:"copyright"`
	ReleaseType         types.String `tfsdk:"release_type"`
	EarliestReleaseDate types.String `tfsdk:"earliest_release_date"`
	ReviewType          types.String `tfsdk:"review_type"`
	UsesIdfa            types.Bool   `tfsdk:"uses_idfa"`
	AppVersionState     types.String `tfsdk:"app_version_state"`
	Downloadable        types.Bool   `tfsdk:"downloadable"`
	CreatedDate         types.String `tfsdk:"created_date"`
}

// appStoreVersionResourceModel is the version plus the build attached to it.
//
// The build lives only on the resource. A listing cannot report it without one
// request per version -- Apple publishes no way to read the attached build of
// many versions at once -- and the two numbers that name a build are inputs
// rather than anything Apple reports on a version, so the data source keeps the
// narrower model it always had.
type appStoreVersionResourceModel struct {
	appStoreVersionModel

	BuildNumber       types.String `tfsdk:"build_number"`
	PreReleaseVersion types.String `tfsdk:"pre_release_version"`
	BuildID           types.String `tfsdk:"build_id"`
}

// trainVersion is the marketing version of the build train this version wants,
// which defaults to the version's own version string.
//
// Apple only offers a version the builds whose CFBundleShortVersionString
// matches it, so the two are the same value in every ordinary configuration and
// pre_release_version exists to be left unset.
func (m *appStoreVersionResourceModel) trainVersion() string {
	if !m.PreReleaseVersion.IsNull() && !m.PreReleaseVersion.IsUnknown() {
		return m.PreReleaseVersion.ValueString()
	}

	return m.VersionString.ValueString()
}

// appStoreVersionLocalizationModel maps a version's localized product page.
//
// Keywords is a list here and a comma-separated string on the wire: Apple has
// never modelled it as a list, but a configuration that has to maintain its own
// comma-joining is worse than one that does not.
type appStoreVersionLocalizationModel struct {
	ID                types.String `tfsdk:"id"`
	AppStoreVersionID types.String `tfsdk:"app_store_version_id"`
	Locale            types.String `tfsdk:"locale"`
	Description       types.String `tfsdk:"description"`
	Keywords          types.List   `tfsdk:"keywords"`
	PromotionalText   types.String `tfsdk:"promotional_text"`
	MarketingURL      types.String `tfsdk:"marketing_url"`
	SupportURL        types.String `tfsdk:"support_url"`
	WhatsNew          types.String `tfsdk:"whats_new"`
}

// appStoreReviewDetailModel maps the review contact and demo account.
type appStoreReviewDetailModel struct {
	ID                  types.String `tfsdk:"id"`
	AppStoreVersionID   types.String `tfsdk:"app_store_version_id"`
	ContactFirstName    types.String `tfsdk:"contact_first_name"`
	ContactLastName     types.String `tfsdk:"contact_last_name"`
	ContactPhone        types.String `tfsdk:"contact_phone"`
	ContactEmail        types.String `tfsdk:"contact_email"`
	DemoAccountRequired types.Bool   `tfsdk:"demo_account_required"`
	DemoAccountName     types.String `tfsdk:"demo_account_name"`
	DemoAccountPassword types.String `tfsdk:"demo_account_password"`
	Notes               types.String `tfsdk:"notes"`
}

// ageRatingDeclarationModel maps the answered content questionnaire.
//
// The members are grouped the way Apple groups the questions: how often a kind
// of content appears, yes/no facts about what the app does, and the overrides
// that raise the computed rating.
type ageRatingDeclarationModel struct {
	ID        types.String `tfsdk:"id"`
	AppID     types.String `tfsdk:"app_id"`
	AppInfoID types.String `tfsdk:"app_info_id"`

	AlcoholTobaccoOrDrugUseOrReferences         types.String `tfsdk:"alcohol_tobacco_or_drug_use_or_references"`
	Contests                                    types.String `tfsdk:"contests"`
	GamblingSimulated                           types.String `tfsdk:"gambling_simulated"`
	GunsOrOtherWeapons                          types.String `tfsdk:"guns_or_other_weapons"`
	HorrorOrFearThemes                          types.String `tfsdk:"horror_or_fear_themes"`
	MatureOrSuggestiveThemes                    types.String `tfsdk:"mature_or_suggestive_themes"`
	MedicalOrTreatmentInformation               types.String `tfsdk:"medical_or_treatment_information"`
	ProfanityOrCrudeHumor                       types.String `tfsdk:"profanity_or_crude_humor"`
	SexualContentGraphicAndNudity               types.String `tfsdk:"sexual_content_graphic_and_nudity"`
	SexualContentOrNudity                       types.String `tfsdk:"sexual_content_or_nudity"`
	ViolenceCartoonOrFantasy                    types.String `tfsdk:"violence_cartoon_or_fantasy"`
	ViolenceRealistic                           types.String `tfsdk:"violence_realistic"`
	ViolenceRealisticProlongedGraphicOrSadistic types.String `tfsdk:"violence_realistic_prolonged_graphic_or_sadistic"`

	Advertising              types.Bool `tfsdk:"advertising"`
	AgeAssurance             types.Bool `tfsdk:"age_assurance"`
	Gambling                 types.Bool `tfsdk:"gambling"`
	HealthOrWellnessTopics   types.Bool `tfsdk:"health_or_wellness_topics"`
	LootBox                  types.Bool `tfsdk:"loot_box"`
	MessagingAndChat         types.Bool `tfsdk:"messaging_and_chat"`
	ParentalControls         types.Bool `tfsdk:"parental_controls"`
	SocialMedia              types.Bool `tfsdk:"social_media"`
	SocialMediaAgeRestricted types.Bool `tfsdk:"social_media_age_restricted"`
	UnrestrictedWebAccess    types.Bool `tfsdk:"unrestricted_web_access"`
	UserGeneratedContent     types.Bool `tfsdk:"user_generated_content"`

	KidsAgeBand               types.String `tfsdk:"kids_age_band"`
	AgeRatingOverride         types.String `tfsdk:"age_rating_override"`
	AgeRatingOverrideV2       types.String `tfsdk:"age_rating_override_v2"`
	KoreaAgeRatingOverride    types.String `tfsdk:"korea_age_rating_override"`
	DeveloperAgeRatingInfoURL types.String `tfsdk:"developer_age_rating_info_url"`
}

// appPriceModel maps one manual price inside the price schedule.
type appPriceModel struct {
	PricePointID types.String `tfsdk:"price_point_id"`
	StartDate    types.String `tfsdk:"start_date"`
	EndDate      types.String `tfsdk:"end_date"`
}

// appPriceScheduleModel maps an app's whole price configuration.
type appPriceScheduleModel struct {
	ID            types.String    `tfsdk:"id"`
	AppID         types.String    `tfsdk:"app_id"`
	BaseTerritory types.String    `tfsdk:"base_territory"`
	Prices        []appPriceModel `tfsdk:"prices"`
}

// appTerritoryAvailabilityModel maps one storefront's availability entry.
type appTerritoryAvailabilityModel struct {
	Territory       types.String `tfsdk:"territory"`
	ReleaseDate     types.String `tfsdk:"release_date"`
	PreOrderEnabled types.Bool   `tfsdk:"pre_order_enabled"`
}

// appAvailabilityModel maps an app's territory availability.
type appAvailabilityModel struct {
	ID                        types.String                    `tfsdk:"id"`
	AppID                     types.String                    `tfsdk:"app_id"`
	AvailableInNewTerritories types.Bool                      `tfsdk:"available_in_new_territories"`
	Territories               []appTerritoryAvailabilityModel `tfsdk:"territories"`
}

// appCategoryModel maps one entry of Apple's category catalogue.
type appCategoryModel struct {
	ID            types.String   `tfsdk:"id"`
	Platforms     []types.String `tfsdk:"platforms"`
	ParentID      types.String   `tfsdk:"parent_id"`
	Subcategories []types.String `tfsdk:"subcategories"`
}

// appCategoriesDataSourceModel maps the category catalogue lookup.
type appCategoriesDataSourceModel struct {
	// Filter configuration. Platforms are passed to Apple rather than applied
	// in memory: Apple publishes filter[platforms] for this collection.
	Platforms types.List `tfsdk:"platforms"`

	// TopLevelOnly drops subcategories, leaving the categories that can be
	// assigned as a primary or secondary category.
	TopLevelOnly types.Bool   `tfsdk:"top_level_only"`
	IDPattern    types.String `tfsdk:"id_pattern"`

	// Result control
	Limit types.Int64 `tfsdk:"limit"`

	// Output
	Categories []appCategoryModel `tfsdk:"categories"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// appPricePointModel maps one entry of Apple's app price catalogue.
type appPricePointModel struct {
	ID            types.String `tfsdk:"id"`
	CustomerPrice types.String `tfsdk:"customer_price"`
	Proceeds      types.String `tfsdk:"proceeds"`
	TerritoryID   types.String `tfsdk:"territory_id"`
}

// appPricePointsDataSourceModel maps the price catalogue lookup.
type appPricePointsDataSourceModel struct {
	// Scope
	AppID types.String `tfsdk:"app_id"`

	// Filter configuration. Territories are passed to Apple rather than
	// applied in memory: the unfiltered catalogue covers every territory the
	// App Store sells in.
	Territories   types.List   `tfsdk:"territories"`
	CustomerPrice types.String `tfsdk:"customer_price"`

	// Result control
	Limit types.Int64 `tfsdk:"limit"`

	// Output
	PricePoints []appPricePointModel `tfsdk:"price_points"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// appStoreVersionsDataSourceModel maps the version listing data source.
type appStoreVersionsDataSourceModel struct {
	// Scope
	AppID types.String `tfsdk:"app_id"`

	// Filter configuration. Platform is passed to Apple; the rest are applied
	// in memory.
	Platform             types.String `tfsdk:"platform"`
	AppVersionState      types.String `tfsdk:"app_version_state"`
	VersionStringPattern types.String `tfsdk:"version_string_pattern"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	Versions []appStoreVersionModel `tfsdk:"versions"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// App metadata validators.
var (
	// ContentRightsValidator validates the Content Rights Information answer.
	ContentRightsValidator = stringvalidator.OneOf(models.ValidContentRightsDeclarations...)

	// SubscriptionStatusURLVersionValidator validates the server notification
	// version.
	SubscriptionStatusURLVersionValidator = stringvalidator.OneOf(models.ValidSubscriptionStatusURLVersions...)

	// AppStorePlatformValidator validates an App Store version platform.
	//
	// This is not the Bundle ID platform list: Apple accepts TV_OS and
	// VISION_OS for a version and rejects both for a Bundle ID.
	AppStorePlatformValidator = stringvalidator.OneOf(models.ValidAppStorePlatforms...)

	// ReleaseTypeValidator validates how an approved version is released.
	ReleaseTypeValidator = stringvalidator.OneOf(models.ValidReleaseTypes...)

	// ReviewTypeValidator validates the review type.
	ReviewTypeValidator = stringvalidator.OneOf(models.ValidReviewTypes...)

	// AppVersionStateValidator validates a version state filter.
	AppVersionStateValidator = stringvalidator.OneOf(models.ValidAppVersionStates...)

	// AgeRatingFrequencyValidator validates a content frequency answer.
	AgeRatingFrequencyValidator = stringvalidator.OneOf(models.ValidAgeRatingFrequencies...)

	// KidsAgeBandValidator validates a Kids Category band.
	KidsAgeBandValidator = stringvalidator.OneOf(models.ValidKidsAgeBands...)

	// AgeRatingOverrideValidator validates the original override values.
	AgeRatingOverrideValidator = stringvalidator.OneOf(models.ValidAgeRatingOverrides...)

	// AgeRatingOverrideV2Validator validates the revised override values.
	AgeRatingOverrideV2Validator = stringvalidator.OneOf(models.ValidAgeRatingOverridesV2...)

	// KoreaAgeRatingOverrideValidator validates the Korea override values.
	KoreaAgeRatingOverrideValidator = stringvalidator.OneOf(models.ValidKoreaAgeRatingOverrides...)

	// LocaleValidator validates an App Store locale code.
	//
	// Apple's locales are BCP 47-ish but not uniformly so: "en-US", "es-MX",
	// "ar-SA", but also bare "ar" and "no". Both shapes are accepted.
	LocaleValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`),
		"Locale must be an App Store locale code such as en-US, es-MX, ar-SA or ar",
	)

	// TerritoryValidator validates a territory code.
	//
	// Apple identifies territories by three-letter uppercase code -- USA, GBR,
	// EGY -- not the two-letter ISO 3166-1 alpha-2 codes.
	TerritoryValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[A-Z]{3}$`),
		"Territory must be a three-letter uppercase Apple territory code such as USA, GBR or EGY",
	)

	// CategoryValidator validates an App Store category identifier.
	//
	// A category ID is the constant itself -- FINANCE, PRODUCTIVITY,
	// GAMES_PUZZLE -- so this only checks the shape. Read the valid values from
	// the apple_app_categories data source; Apple rejects an unknown one.
	CategoryValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`),
		"Category must be an Apple category constant such as FINANCE, PRODUCTIVITY or GAMES_PUZZLE",
	)

	// DateValidator validates the plain date Apple expects on a price window.
	DateValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`),
		"Date must be a plain date in YYYY-MM-DD form, not a timestamp",
	)

	// TimestampValidator validates the ISO 8601 timestamp Apple expects on a
	// scheduled release.
	//
	// Unlike a price window, which takes a plain date, earliestReleaseDate is a
	// full timestamp and Apple requires the seconds and the zone.
	TimestampValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}([+-]\d{2}:\d{2}|Z)$`),
		"Timestamp must be ISO 8601 with seconds and a zone offset, such as 2026-03-01T08:00:00-07:00",
	)

	// PriceValidator validates a customer price filter.
	PriceValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^\d+(\.\d+)?$`),
		"Customer price must be a plain decimal number without a currency symbol, such as 9.99",
	)

	// EmailValidator validates the App Review contact address.
	//
	// Deliberately loose: it checks that the value looks like an address rather
	// than trying to decide which addresses are real, because rejecting one
	// Apple would have accepted is worse than passing it through.
	EmailValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`),
		"Contact email must look like an email address, such as review@example.com",
	)

	// URLValidator validates the marketing, support and privacy policy links.
	URLValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^https?://`),
		"URL must start with http:// or https://",
	)

	// VersionSortByValidator validates sort field options for versions.
	VersionSortByValidator = stringvalidator.OneOf("version_string", "created_date", "app_version_state", "platform")
)

// GetAppNameValidator returns validators for the customer-facing app name.
//
// Thirty characters, counted in characters rather than bytes: UTF8LengthBetween
// and not LengthBetween, because the latter counts bytes and rejects a legal
// Arabic or Japanese name at roughly half the length Apple allows.
func GetAppNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 30),
	}
}

// GetSubtitleValidator returns validators for the app subtitle.
//
// Counted in characters -- see GetAppNameValidator.
func GetSubtitleValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 30),
	}
}

// GetVersionDescriptionValidator returns validators for the product page
// description.
//
// Counted in characters -- see GetAppNameValidator.
func GetVersionDescriptionValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 4000),
	}
}

// GetPromotionalTextValidator returns validators for promotional text.
//
// Counted in characters -- see GetAppNameValidator.
func GetPromotionalTextValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 170),
	}
}

// GetWhatsNewValidator returns validators for the release notes.
//
// Counted in characters -- see GetAppNameValidator.
func GetWhatsNewValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 4000),
	}
}

// GetReviewNotesValidator returns validators for the App Review notes.
func GetReviewNotesValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 4000),
	}
}

// GetCopyrightValidator returns validators for the copyright line.
func GetCopyrightValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 200),
	}
}

// GetVersionStringValidator returns validators for a version string.
//
// Apple wants up to three dot-separated numbers -- "1.0", "2.14.3" -- and
// rejects a leading zero component or a fourth field.
func GetVersionStringValidator() []validator.String {
	return []validator.String{
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^\d+(\.\d+){0,2}$`),
			"Version string must be up to three dot-separated numbers, such as 1.0 or 2.14.3",
		),
	}
}

// GetBuildNumberValidator returns validators for a build number.
//
// This is CFBundleVersion, which Apple requires to be dot-separated integers --
// the count is not capped here, because rejecting a build number Apple already
// accepted at upload would leave a configuration with no way to name its own
// build. It catches the shape that is actually wrong: a "v" prefix, a git SHA,
// or the marketing version pasted in by mistake.
func GetBuildNumberValidator() []validator.String {
	return []validator.String{
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^\d+(\.\d+)*$`),
			"Build number must be dot-separated numbers, such as 42 or 1.2.3",
		),
	}
}

// keywordsMaxLength is Apple's ceiling on the comma-joined keyword string.
//
// The limit applies to the joined string, commas included, not to any single
// keyword -- which is why it is enforced after joining rather than per element.
const keywordsMaxLength = 100

// joinKeywords renders a keyword list as the comma-separated string Apple
// stores.
//
// No space after the comma: a space is a character, and every one of them comes
// out of the hundred a locale gets.
//
// The parameter is a types.List rather than a []types.String because the whole
// list can be unknown -- keywords supplied from a variable or a for_each are
// unknown at plan time -- and a Go slice has no way to represent that. An
// unknown or null list joins to the empty string, which the callers treat as
// "nothing configured".
func joinKeywords(keywords types.List) string {
	if keywords.IsNull() || keywords.IsUnknown() {
		return ""
	}

	parts := make([]string, 0, len(keywords.Elements()))
	for _, element := range keywords.Elements() {
		keyword, ok := element.(types.String)
		if !ok || keyword.IsNull() || keyword.IsUnknown() {
			continue
		}
		if trimmed := strings.TrimSpace(keyword.ValueString()); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}

	return strings.Join(parts, ",")
}

// splitKeywords parses Apple's comma-separated keyword string back into a list.
//
// Apple echoes back what was sent, but a keyword list edited in App Store
// Connect comes back with spaces after the commas, so each element is trimmed.
func splitKeywords(keywords *string) types.List {
	if keywords == nil || strings.TrimSpace(*keywords) == "" {
		return types.ListNull(types.StringType)
	}

	parts := strings.Split(*keywords, ",")
	elements := make([]attr.Value, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			elements = append(elements, types.StringValue(trimmed))
		}
	}

	if len(elements) == 0 {
		return types.ListNull(types.StringType)
	}

	// The elements are known strings, so the conversion cannot fail; the
	// diagnostics are discarded rather than plumbed through every caller.
	list, _ := types.ListValue(types.StringType, elements)

	return list
}

// stringListValues reads a configured list attribute into a Go slice, dropping
// null and unknown elements.
//
// Used for the filter attributes the data sources pass to Apple. A list that is
// itself null or unknown yields nothing, which the callers treat as no filter.
func stringListValues(list types.List) []string {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}

	values := make([]string, 0, len(list.Elements()))
	for _, element := range list.Elements() {
		value, ok := element.(types.String)
		if !ok || value.IsNull() || value.IsUnknown() {
			continue
		}
		values = append(values, value.ValueString())
	}

	return values
}

// stringOrNil converts an optional Terraform string to the pointer the API
// models use, so an unset attribute is omitted from the request rather than
// sent as an empty string.
func stringOrNil(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()

	return &s
}

// boolOrNil converts an optional Terraform bool to a pointer.
func boolOrNil(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()

	return &b
}

// stringOrNull converts an optional API string pointer back to Terraform,
// mapping a missing value to null rather than to "".
func stringOrNull(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}

	return types.StringValue(*s)
}

// boolOrNull converts an optional API bool pointer back to Terraform.
func boolOrNull(b *bool) types.Bool {
	if b == nil {
		return types.BoolNull()
	}

	return types.BoolValue(*b)
}

// relationshipID reads an ID out of a to-one relationship, returning null when
// Apple did not populate the linkage.
func relationshipID(rel *models.ResourceIdentifier) types.String {
	if rel == nil || rel.Data.ID == "" {
		return types.StringNull()
	}

	return types.StringValue(rel.Data.ID)
}

// categoryLinkage renders a configured category as the relationship member
// Apple expects.
//
// An unset category is sent as an explicit null rather than omitted: omitting
// it would leave whatever is already there, so a category removed from the
// configuration would never actually be removed from the app.
func categoryLinkage(v types.String) *models.NullableResourceIdentifier {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return &models.NullableResourceIdentifier{Data: nil}
	}

	return &models.NullableResourceIdentifier{
		Data: &models.ResourceData{Type: "appCategories", ID: v.ValueString()},
	}
}
