// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// App info state enumeration.
//
// An app carries several AppInfo records at once: the one backing the version
// on sale, and the editable one holding what the next submission will say.
// Only the editable record accepts a write, which is why the provider selects
// by state rather than taking the first record Apple returns.
type AppInfoState string

const (
	AppInfoStatePrepareForSubmission AppInfoState = "PREPARE_FOR_SUBMISSION"
	AppInfoStateDeveloperRejected    AppInfoState = "DEVELOPER_REJECTED"
	AppInfoStateRejected             AppInfoState = "REJECTED"
	AppInfoStateReadyForReview       AppInfoState = "READY_FOR_REVIEW"
	AppInfoStateWaitingForReview     AppInfoState = "WAITING_FOR_REVIEW"
	AppInfoStateInReview             AppInfoState = "IN_REVIEW"
	AppInfoStateAccepted             AppInfoState = "ACCEPTED"
	AppInfoStatePendingRelease       AppInfoState = "PENDING_RELEASE"
	AppInfoStateReadyForDistribution AppInfoState = "READY_FOR_DISTRIBUTION"
	AppInfoStateReplacedWithNewInfo  AppInfoState = "REPLACED_WITH_NEW_INFO"
)

// EditableAppInfoStates lists the states in which Apple accepts a write to an
// AppInfo or the localizations and age rating hanging off it.
//
// A record in review or already distributed is frozen. Unlike an in-app
// purchase version, an AppInfo cannot be created to get around that -- Apple
// publishes no POST /v1/appInfos -- so a write outside these states fails and
// the caller is told to wait for review to finish.
var EditableAppInfoStates = []AppInfoState{
	AppInfoStatePrepareForSubmission,
	AppInfoStateDeveloperRejected,
	AppInfoStateRejected,
	AppInfoStateReadyForReview,
}

// AppInfo is the part of an app's metadata that outlives any single version.
//
// Categories, the customer-facing name and subtitle, the privacy policy URL and
// the age rating all hang off this rather than off an AppStoreVersion: they
// describe the app, not the release.
type AppInfo struct {
	Type          string                `json:"type"`
	ID            string                `json:"id"`
	Attributes    AppInfoAttributes     `json:"attributes"`
	Relationships *AppInfoRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks        `json:"links,omitempty"`
}

// AppInfoAttributes are all Apple's: every one is derived or assigned, and none
// of them appears in the update request.
type AppInfoAttributes struct {
	State              *AppInfoState `json:"state,omitempty"`
	AppStoreState      *string       `json:"appStoreState,omitempty"`
	AppStoreAgeRating  *string       `json:"appStoreAgeRating,omitempty"`
	BrazilAgeRatingV2  *string       `json:"brazilAgeRatingV2,omitempty"`
	KoreaAgeRating     *string       `json:"koreaAgeRating,omitempty"`
	AustraliaAgeRating *string       `json:"australiaAgeRating,omitempty"`
	FranceAgeRating    *string       `json:"franceAgeRating,omitempty"`
	KidsAgeBand        *string       `json:"kidsAgeBand,omitempty"`
}

// AppInfoRelationships holds the linkages Apple reports.
//
// The six category members are the whole of App Store Connect's category
// picker: a primary category with up to two subcategories, and an optional
// secondary category with up to two of its own.
type AppInfoRelationships struct {
	App                     *ResourceIdentifier `json:"app,omitempty"`
	AgeRatingDeclaration    *ResourceIdentifier `json:"ageRatingDeclaration,omitempty"`
	PrimaryCategory         *ResourceIdentifier `json:"primaryCategory,omitempty"`
	PrimarySubcategoryOne   *ResourceIdentifier `json:"primarySubcategoryOne,omitempty"`
	PrimarySubcategoryTwo   *ResourceIdentifier `json:"primarySubcategoryTwo,omitempty"`
	SecondaryCategory       *ResourceIdentifier `json:"secondaryCategory,omitempty"`
	SecondarySubcategoryOne *ResourceIdentifier `json:"secondarySubcategoryOne,omitempty"`
	SecondarySubcategoryTwo *ResourceIdentifier `json:"secondarySubcategoryTwo,omitempty"`
}

// NullableResourceIdentifier is a to-one relationship that can be cleared.
//
// JSON:API removes a relationship by sending an explicit null under "data", so
// this cannot use ResourceIdentifier: that type would marshal an unset value as
// {"type":"","id":""}, which Apple rejects. A nil Data marshals as
// {"data":null}, which is how a category is removed.
type NullableResourceIdentifier struct {
	Data *ResourceData `json:"data"`
}

// AppInfoUpdateRequest sets an app's categories.
//
// The request carries no attributes at all -- everything on an AppInfo that can
// be written is written through a relationship. A member left nil is omitted
// and the existing value is kept; a member present with a null Data clears the
// category.
type AppInfoUpdateRequest struct {
	Type          string                     `json:"type"`
	ID            string                     `json:"id"`
	Relationships AppInfoUpdateRelationships `json:"relationships"`
}

type AppInfoUpdateRelationships struct {
	PrimaryCategory         *NullableResourceIdentifier `json:"primaryCategory,omitempty"`
	PrimarySubcategoryOne   *NullableResourceIdentifier `json:"primarySubcategoryOne,omitempty"`
	PrimarySubcategoryTwo   *NullableResourceIdentifier `json:"primarySubcategoryTwo,omitempty"`
	SecondaryCategory       *NullableResourceIdentifier `json:"secondaryCategory,omitempty"`
	SecondarySubcategoryOne *NullableResourceIdentifier `json:"secondarySubcategoryOne,omitempty"`
	SecondarySubcategoryTwo *NullableResourceIdentifier `json:"secondarySubcategoryTwo,omitempty"`
}

// AppCategory is one entry in Apple's fixed category catalogue.
//
// The ID is the category name as a constant -- FINANCE, PRODUCTIVITY,
// GAMES_PUZZLE -- so a category relationship can be written without looking
// anything up. Categories belong to the App Store rather than to the account,
// so there is no resource and cannot be one; the apple_app_categories data
// source exists to enumerate the valid IDs and which platforms accept them.
type AppCategory struct {
	Type          string                    `json:"type"`
	ID            string                    `json:"id"`
	Attributes    AppCategoryAttributes     `json:"attributes"`
	Relationships *AppCategoryRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks            `json:"links,omitempty"`
}

type AppCategoryAttributes struct {
	Platforms []string `json:"platforms,omitempty"`
}

type AppCategoryRelationships struct {
	Parent        *ResourceIdentifier  `json:"parent,omitempty"`
	Subcategories *ResourceIdentifiers `json:"subcategories,omitempty"`
}

// AppInfoLocalization is the customer-facing name of the app itself.
//
// This is the half of the localized metadata that survives a release: the app
// name, its subtitle, and the privacy policy link. The description, keywords
// and promotional text belong to a single version and live on an
// AppStoreVersionLocalization instead.
type AppInfoLocalization struct {
	Type          string                            `json:"type"`
	ID            string                            `json:"id"`
	Attributes    AppInfoLocalizationAttributes     `json:"attributes"`
	Relationships *AppInfoLocalizationRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                    `json:"links,omitempty"`
}

type AppInfoLocalizationAttributes struct {
	Locale            string  `json:"locale,omitempty"`
	Name              string  `json:"name,omitempty"`
	Subtitle          *string `json:"subtitle,omitempty"`
	PrivacyPolicyURL  *string `json:"privacyPolicyUrl,omitempty"`
	PrivacyChoicesURL *string `json:"privacyChoicesUrl,omitempty"`
	PrivacyPolicyText *string `json:"privacyPolicyText,omitempty"`
}

type AppInfoLocalizationRelationships struct {
	AppInfo *ResourceIdentifier `json:"appInfo,omitempty"`
}

type AppInfoLocalizationCreateRequest struct {
	Type          string                                 `json:"type"`
	Attributes    AppInfoLocalizationCreateAttributes    `json:"attributes"`
	Relationships AppInfoLocalizationCreateRelationships `json:"relationships"`
}

type AppInfoLocalizationCreateAttributes struct {
	Locale            string  `json:"locale"`
	Name              string  `json:"name"`
	Subtitle          *string `json:"subtitle,omitempty"`
	PrivacyPolicyURL  *string `json:"privacyPolicyUrl,omitempty"`
	PrivacyChoicesURL *string `json:"privacyChoicesUrl,omitempty"`
	PrivacyPolicyText *string `json:"privacyPolicyText,omitempty"`
}

type AppInfoLocalizationCreateRelationships struct {
	AppInfo ResourceIdentifier `json:"appInfo"`
}

type AppInfoLocalizationUpdateRequest struct {
	Type       string                              `json:"type"`
	ID         string                              `json:"id"`
	Attributes AppInfoLocalizationUpdateAttributes `json:"attributes"`
}

// AppInfoLocalizationUpdateAttributes omits locale: it identifies the record
// and is absent from Apple's update request, the same shape every other
// localization in this provider has.
type AppInfoLocalizationUpdateAttributes struct {
	Name              *string `json:"name,omitempty"`
	Subtitle          *string `json:"subtitle,omitempty"`
	PrivacyPolicyURL  *string `json:"privacyPolicyUrl,omitempty"`
	PrivacyChoicesURL *string `json:"privacyChoicesUrl,omitempty"`
	PrivacyPolicyText *string `json:"privacyPolicyText,omitempty"`
}
