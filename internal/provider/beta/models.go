// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package beta

import (
	"regexp"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// betaGroupModel maps one TestFlight tester group.
//
// is_internal_group and has_access_to_all_builds are inputs Apple accepts only
// at creation; public_link, public_link_id and created_date are Apple's alone.
type betaGroupModel struct {
	ID                                   types.String `tfsdk:"id"`
	AppID                                types.String `tfsdk:"app_id"`
	Name                                 types.String `tfsdk:"name"`
	IsInternalGroup                      types.Bool   `tfsdk:"is_internal_group"`
	HasAccessToAllBuilds                 types.Bool   `tfsdk:"has_access_to_all_builds"`
	PublicLinkEnabled                    types.Bool   `tfsdk:"public_link_enabled"`
	PublicLinkLimitEnabled               types.Bool   `tfsdk:"public_link_limit_enabled"`
	PublicLinkLimit                      types.Int64  `tfsdk:"public_link_limit"`
	FeedbackEnabled                      types.Bool   `tfsdk:"feedback_enabled"`
	IOSBuildsAvailableForAppleSiliconMac types.Bool   `tfsdk:"ios_builds_available_for_apple_silicon_mac"`
	IOSBuildsAvailableForAppleVision     types.Bool   `tfsdk:"ios_builds_available_for_apple_vision"`
	PublicLink                           types.String `tfsdk:"public_link"`
	PublicLinkID                         types.String `tfsdk:"public_link_id"`
	CreatedDate                          types.String `tfsdk:"created_date"`
}

// betaAppLocalizationModel maps an app's TestFlight page text in one language.
//
// It hangs off the app and survives every build, which is what separates it
// from betaBuildLocalizationModel.
type betaAppLocalizationModel struct {
	ID                types.String `tfsdk:"id"`
	AppID             types.String `tfsdk:"app_id"`
	Locale            types.String `tfsdk:"locale"`
	Description       types.String `tfsdk:"description"`
	FeedbackEmail     types.String `tfsdk:"feedback_email"`
	MarketingURL      types.String `tfsdk:"marketing_url"`
	PrivacyPolicyURL  types.String `tfsdk:"privacy_policy_url"`
	TVOSPrivacyPolicy types.String `tfsdk:"tvos_privacy_policy"`
}

// betaBuildLocalizationModel maps one build's "What to Test" note.
//
// The build is named by the two numbers a pipeline already exports rather than
// by Apple's opaque ID, the same way apple_app_store_version names it: the
// record hangs off a build, and nobody has a build ID to hand.
type betaBuildLocalizationModel struct {
	ID                types.String `tfsdk:"id"`
	AppID             types.String `tfsdk:"app_id"`
	Platform          types.String `tfsdk:"platform"`
	PreReleaseVersion types.String `tfsdk:"pre_release_version"`
	BuildNumber       types.String `tfsdk:"build_number"`
	BuildID           types.String `tfsdk:"build_id"`
	Locale            types.String `tfsdk:"locale"`
	WhatsNew          types.String `tfsdk:"whats_new"`
}

// betaTesterModel maps one tester's membership of one TestFlight group.
//
// The Apple record behind id is the account's rather than the group's -- one
// tester record per email address, shared by every group it belongs to -- so a
// tester in three groups is three of these resources carrying the same id. What
// each resource owns is the membership, which is what Delete withdraws.
type betaTesterModel struct {
	ID         types.String `tfsdk:"id"`
	GroupID    types.String `tfsdk:"group_id"`
	Email      types.String `tfsdk:"email"`
	FirstName  types.String `tfsdk:"first_name"`
	LastName   types.String `tfsdk:"last_name"`
	InviteType types.String `tfsdk:"invite_type"`
}

// betaAppReviewDetailModel maps what the beta reviewers are told about an app.
type betaAppReviewDetailModel struct {
	ID                  types.String `tfsdk:"id"`
	AppID               types.String `tfsdk:"app_id"`
	ContactFirstName    types.String `tfsdk:"contact_first_name"`
	ContactLastName     types.String `tfsdk:"contact_last_name"`
	ContactPhone        types.String `tfsdk:"contact_phone"`
	ContactEmail        types.String `tfsdk:"contact_email"`
	DemoAccountName     types.String `tfsdk:"demo_account_name"`
	DemoAccountPassword types.String `tfsdk:"demo_account_password"`
	DemoAccountRequired types.Bool   `tfsdk:"demo_account_required"`
	Notes               types.String `tfsdk:"notes"`
}

var (
	// LocaleValidator validates a TestFlight locale code.
	//
	// The same shape the App Store side accepts: mostly BCP 47 -- "en-US",
	// "es-MX", "ar-SA" -- but also bare "ar" and "no".
	LocaleValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`),
		"Locale must be an App Store locale code such as en-US, es-MX, ar-SA or ar",
	)

	// EmailValidator validates the beta feedback and review contact addresses.
	//
	// Deliberately loose, for the reason the App Store one is: rejecting an
	// address Apple would have accepted is worse than passing it through.
	EmailValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`),
		"Email must look like an email address, such as testflight@example.com",
	)

	// URLValidator validates the marketing and privacy policy links.
	URLValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^https?://`),
		"URL must start with http:// or https://",
	)

	// BuildPlatformValidator validates the platform a build's train belongs to.
	//
	// The same list an App Store version accepts, and for the same reason: a
	// build is uploaded per platform, and TV_OS and VISION_OS are both real
	// here where neither is a valid Bundle ID platform.
	BuildPlatformValidator = stringvalidator.OneOf(models.ValidAppStorePlatforms...)
)

// GetBetaGroupNameValidator returns validators for a group's name.
//
// Apple enforces the name unique within an app. The ceiling is generous rather
// than exact: the useful check is that a name was actually given.
func GetBetaGroupNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 255),
	}
}

// GetBetaTesterNameValidator returns validators for a tester's first or last
// name.
//
// Counted in characters rather than bytes, like every other name this provider
// sends to Apple. The ceiling is generous rather than exact: the useful check is
// that an empty string is not sent as a name.
func GetBetaTesterNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 255),
	}
}

// GetBetaDescriptionValidator returns validators for the TestFlight page
// description.
//
// Counted in characters rather than bytes -- UTF8LengthBetween and not
// LengthBetween, because the latter counts bytes and rejects a legal Arabic or
// Japanese description at roughly half the length Apple allows.
func GetBetaDescriptionValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 4000),
	}
}

// GetWhatsNewValidator returns validators for a build's "What to Test" note.
//
// Counted in characters -- see GetBetaDescriptionValidator.
func GetWhatsNewValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 4000),
	}
}

// GetReviewNotesValidator returns validators for the beta review notes.
func GetReviewNotesValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 4000),
	}
}

// GetPublicLinkLimitValidator returns validators for the tester cap on a public
// link.
//
// A cap of zero is not a closed link, it is a link Apple rejects; closing one
// is public_link_enabled = false.
func GetPublicLinkLimitValidator() []validator.Int64 {
	return []validator.Int64{
		int64validator.AtLeast(1),
	}
}

// GetVersionStringValidator returns validators for a build train's marketing
// version -- CFBundleShortVersionString, up to three dot-separated numbers.
func GetVersionStringValidator() []validator.String {
	return []validator.String{
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^\d+(\.\d+){0,2}$`),
			"Pre-release version must be up to three dot-separated numbers, such as 1.0 or 2.14.3",
		),
	}
}

// GetBuildNumberValidator returns validators for a build number.
//
// This is CFBundleVersion, which Apple requires to be dot-separated integers.
// The count is not capped: rejecting a build number Apple already accepted at
// upload would leave a configuration with no way to name its own build. It
// catches the shape that is actually wrong -- a "v" prefix, a git SHA, or the
// marketing version pasted in by mistake.
func GetBuildNumberValidator() []validator.String {
	return []validator.String{
		stringvalidator.RegexMatches(
			regexp.MustCompile(`^\d+(\.\d+)*$`),
			"Build number must be dot-separated numbers, such as 42 or 1.2.3",
		),
	}
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

// int64OrNil converts an optional Terraform integer to a pointer.
func int64OrNil(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()

	return &i
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
