// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package inapppurchase contains the one-time purchase resources and data
// sources for the Apple Terraform provider: in-app purchases, their
// localizations, their price schedule and their territory availability.
//
// These are the consumables, non-consumables and non-renewing subscriptions of
// the App Store, and they are a different Apple resource from the
// auto-renewable subscriptions in the subscription package -- Apple models the
// two separately, and nothing is shared between them, not even price points.
//
// Three things shape every type here:
//
// They hang off an app record, which Apple's API cannot create -- "Don't use
// this API to create new apps; instead, create new apps on the App Store
// Connect website" -- so the app is always looked up, never managed. Worse than
// a subscription group, an in-app purchase never reports which app owns it:
// InAppPurchaseV2 has no app relationship at all, so app_id is write-once and
// unreadable and import takes a composite form.
//
// Localizations sit behind a version. Apple moved them there in App Store
// Connect API 4.4.1 and deprecated the endpoints that hid it, so a localization
// is created against an in-app purchase version rather than the purchase
// itself. The resource resolves the version for you.
//
// Prices and availability are singular and unnamed. A purchase has one price
// schedule and one availability record, both replaced wholesale by a POST, and
// Apple publishes neither PATCH nor DELETE for either -- which is why those two
// resources update in place and cannot really be destroyed.
package inapppurchase

import (
	"regexp"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// inAppPurchaseModel maps the in-app purchase schema for both the resource and
// the data source.
type inAppPurchaseModel struct {
	ID                types.String `tfsdk:"id"`
	AppID             types.String `tfsdk:"app_id"`
	ProductID         types.String `tfsdk:"product_id"`
	Name              types.String `tfsdk:"name"`
	InAppPurchaseType types.String `tfsdk:"in_app_purchase_type"`
	FamilySharable    types.Bool   `tfsdk:"family_sharable"`
	ReviewNote        types.String `tfsdk:"review_note"`
	State             types.String `tfsdk:"state"`
	ContentHosting    types.Bool   `tfsdk:"content_hosting"`
}

// inAppPurchaseLocalizationModel maps the localization schema.
type inAppPurchaseLocalizationModel struct {
	ID              types.String `tfsdk:"id"`
	InAppPurchaseID types.String `tfsdk:"in_app_purchase_id"`
	VersionID       types.String `tfsdk:"version_id"`
	Locale          types.String `tfsdk:"locale"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
}

// inAppPurchasePriceModel maps one manual price inside the price schedule.
//
// It carries no territory of its own: a price point already belongs to one, and
// naming it separately would only invite the two to disagree.
type inAppPurchasePriceModel struct {
	PricePointID types.String `tfsdk:"price_point_id"`
	StartDate    types.String `tfsdk:"start_date"`
	EndDate      types.String `tfsdk:"end_date"`
}

// inAppPurchasePriceScheduleModel maps the price schedule schema.
type inAppPurchasePriceScheduleModel struct {
	ID              types.String              `tfsdk:"id"`
	InAppPurchaseID types.String              `tfsdk:"in_app_purchase_id"`
	BaseTerritory   types.String              `tfsdk:"base_territory"`
	Prices          []inAppPurchasePriceModel `tfsdk:"prices"`
}

// inAppPurchaseAvailabilityModel maps the availability schema.
type inAppPurchaseAvailabilityModel struct {
	ID                        types.String   `tfsdk:"id"`
	InAppPurchaseID           types.String   `tfsdk:"in_app_purchase_id"`
	AvailableInNewTerritories types.Bool     `tfsdk:"available_in_new_territories"`
	AvailableTerritories      []types.String `tfsdk:"available_territories"`
}

// inAppPurchasePricePointModel maps one entry of Apple's price catalogue.
type inAppPurchasePricePointModel struct {
	ID            types.String `tfsdk:"id"`
	CustomerPrice types.String `tfsdk:"customer_price"`
	Proceeds      types.String `tfsdk:"proceeds"`
	TerritoryID   types.String `tfsdk:"territory_id"`
}

// inAppPurchasesDataSourceModel maps the listing data source.
type inAppPurchasesDataSourceModel struct {
	// Scope
	AppID types.String `tfsdk:"app_id"`

	// Filter configuration
	NamePattern       types.String `tfsdk:"name_pattern"`
	ProductIDPattern  types.String `tfsdk:"product_id_pattern"`
	InAppPurchaseType types.String `tfsdk:"in_app_purchase_type"`
	State             types.String `tfsdk:"state"`

	// Result control
	Limit     types.Int64  `tfsdk:"limit"`
	SortBy    types.String `tfsdk:"sort_by"`
	SortOrder types.String `tfsdk:"sort_order"`

	// Output
	InAppPurchases []inAppPurchaseModel `tfsdk:"in_app_purchases"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// inAppPurchasePricePointsDataSourceModel maps the price catalogue lookup.
type inAppPurchasePricePointsDataSourceModel struct {
	// Scope
	InAppPurchaseID types.String `tfsdk:"in_app_purchase_id"`

	// Filter configuration. Territories are passed to Apple rather than
	// applied in memory: the unfiltered catalogue covers every territory the
	// App Store sells in.
	Territories   []types.String `tfsdk:"territories"`
	CustomerPrice types.String   `tfsdk:"customer_price"`

	// Result control
	Limit types.Int64 `tfsdk:"limit"`

	// Output
	PricePoints []inAppPurchasePricePointModel `tfsdk:"price_points"`

	// Computed metadata
	TotalCount    types.Int64 `tfsdk:"total_count"`
	FilteredCount types.Int64 `tfsdk:"filtered_count"`
}

// In-app purchase validators.
var (
	// ProductIDValidator validates an in-app purchase product identifier.
	//
	// Apple documents no format beyond uniqueness across the account and a
	// 100-character ceiling. Deliberately permissive, for the same reason the
	// subscription package is: rejecting an identifier Apple would have
	// accepted is worse than passing it through and surfacing Apple's error.
	ProductIDValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`),
		"Product ID must start with a letter or digit and contain only letters, digits, dots, underscores and hyphens",
	)

	// TypeValidator validates the kind of one-time purchase.
	TypeValidator = stringvalidator.OneOf(models.ValidInAppPurchaseTypes...)

	// StateValidator validates an in-app purchase state filter.
	StateValidator = stringvalidator.OneOf(models.ValidInAppPurchaseStates...)

	// LocaleValidator validates an App Store locale code.
	//
	// Apple's locales are BCP 47-ish but not uniformly so: "en-US", "es-MX",
	// "ar-SA", but also bare "ar" and "no". Both shapes are accepted.
	LocaleValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`),
		"Locale must be an App Store locale code such as en-US, es-MX, ar-SA or ar",
	)

	// DateValidator validates the plain date Apple expects.
	//
	// A price window takes a date, not a timestamp: "2026-01-01", never
	// "2026-01-01T00:00:00Z".
	DateValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`),
		"Date must be a plain date in YYYY-MM-DD form, not a timestamp",
	)

	// TerritoryValidator validates a territory code.
	//
	// Apple identifies territories by three-letter uppercase code -- USA, GBR,
	// EGY -- not the two-letter ISO 3166-1 alpha-2 codes.
	TerritoryValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^[A-Z]{3}$`),
		"Territory must be a three-letter uppercase Apple territory code such as USA, GBR or EGY",
	)

	// PriceValidator validates a customer price filter.
	//
	// Apple reports prices as decimal strings without a currency symbol, since
	// the currency is a property of the territory: "9.99", "1200", "0.99".
	PriceValidator = stringvalidator.RegexMatches(
		regexp.MustCompile(`^\d+(\.\d+)?$`),
		"Customer price must be a plain decimal number without a currency symbol, such as 9.99",
	)

	// SortByValidator validates sort field options.
	SortByValidator = stringvalidator.OneOf("name", "product_id", "state", "in_app_purchase_type")

	// SortOrderValidator validates sort order options.
	SortOrderValidator = stringvalidator.OneOf("asc", "desc")
)

// GetProductIDValidator returns validators for product identifiers.
func GetProductIDValidator() []validator.String {
	return []validator.String{
		ProductIDValidator,
		stringvalidator.LengthBetween(1, 100),
	}
}

// GetReferenceNameValidator returns validators for the internal reference name.
//
// App Store Connect allows 64 characters here, more than the 30 a customer-
// facing name gets, because nobody outside the account ever reads it.
//
// The limit is counted in characters, so this uses UTF8LengthBetween rather
// than LengthBetween: the latter counts bytes, which rejects a perfectly legal
// Arabic or Japanese name at roughly half the length Apple actually allows.
func GetReferenceNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 64),
	}
}

// GetNameValidator returns validators for the customer-facing name.
//
// Counted in characters, not bytes -- see GetReferenceNameValidator.
func GetNameValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 30),
	}
}

// GetDescriptionValidator returns validators for localization descriptions.
//
// Counted in characters, not bytes -- see GetReferenceNameValidator.
func GetDescriptionValidator() []validator.String {
	return []validator.String{
		stringvalidator.UTF8LengthBetween(1, 45),
	}
}

// GetPatternValidator returns validators for regex pattern fields.
func GetPatternValidator() []validator.String {
	return []validator.String{
		stringvalidator.LengthBetween(1, 255),
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

// stringOrNull converts an optional API string pointer back to Terraform,
// mapping a missing value to null rather than to "".
func stringOrNull(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

// relationshipID reads an ID out of a to-one relationship, returning null when
// Apple did not populate the linkage.
func relationshipID(rel *models.ResourceIdentifier) types.String {
	if rel == nil || rel.Data.ID == "" {
		return types.StringNull()
	}
	return types.StringValue(rel.Data.ID)
}
