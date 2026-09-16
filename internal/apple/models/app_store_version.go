// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Platform enumeration for App Store versions.
//
// This is not the same list as a Bundle ID's platform: an App Store version is
// per-platform and Apple accepts VISION_OS and TV_OS here, neither of which is
// a valid Bundle ID platform.
const (
	AppStorePlatformIOS      = "IOS"
	AppStorePlatformMacOS    = "MAC_OS"
	AppStorePlatformTVOS     = "TV_OS"
	AppStorePlatformVisionOS = "VISION_OS"
)

// ValidAppStorePlatforms lists the platforms an App Store version can target.
var ValidAppStorePlatforms = []string{
	AppStorePlatformIOS,
	AppStorePlatformMacOS,
	AppStorePlatformTVOS,
	AppStorePlatformVisionOS,
}

// Release type enumeration.
//
// MANUAL holds an approved version until it is released by hand,
// AFTER_APPROVAL releases it the moment review passes, and SCHEDULED releases
// it at earliestReleaseDate. This is the attribute an "automatic release"
// setting maps onto.
const (
	ReleaseTypeManual        = "MANUAL"
	ReleaseTypeAfterApproval = "AFTER_APPROVAL"
	ReleaseTypeScheduled     = "SCHEDULED"
)

// ValidReleaseTypes lists the accepted release types.
var ValidReleaseTypes = []string{
	ReleaseTypeManual,
	ReleaseTypeAfterApproval,
	ReleaseTypeScheduled,
}

// ValidReviewTypes lists the accepted review types. NOTARIZATION is for Mac
// software distributed outside the App Store.
var ValidReviewTypes = []string{"APP_STORE", "NOTARIZATION"}

// ValidAppVersionStates lists the states Apple reports for a version.
var ValidAppVersionStates = []string{
	"ACCEPTED",
	"DEVELOPER_REJECTED",
	"IN_REVIEW",
	"INVALID_BINARY",
	"METADATA_REJECTED",
	"PENDING_APPLE_RELEASE",
	"PENDING_DEVELOPER_RELEASE",
	"PREPARE_FOR_SUBMISSION",
	"PROCESSING_FOR_DISTRIBUTION",
	"READY_FOR_DISTRIBUTION",
	"READY_FOR_REVIEW",
	"REJECTED",
	"REPLACED_WITH_NEW_VERSION",
	"WAITING_FOR_EXPORT_COMPLIANCE",
	"WAITING_FOR_REVIEW",
}

// AppStoreVersion is one release of an app on one platform.
//
// It owns everything that describes a particular release rather than the app:
// the version string, the copyright line, how it is released once approved, and
// the localized description, keywords and release notes.
type AppStoreVersion struct {
	Type          string                        `json:"type"`
	ID            string                        `json:"id"`
	Attributes    AppStoreVersionAttributes     `json:"attributes"`
	Relationships *AppStoreVersionRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                `json:"links,omitempty"`
}

type AppStoreVersionAttributes struct {
	Platform            string  `json:"platform,omitempty"`
	VersionString       string  `json:"versionString,omitempty"`
	AppVersionState     *string `json:"appVersionState,omitempty"`
	AppStoreState       *string `json:"appStoreState,omitempty"`
	Copyright           *string `json:"copyright,omitempty"`
	ReviewType          *string `json:"reviewType,omitempty"`
	ReleaseType         *string `json:"releaseType,omitempty"`
	EarliestReleaseDate *string `json:"earliestReleaseDate,omitempty"`
	UsesIdfa            *bool   `json:"usesIdfa,omitempty"`
	Downloadable        *bool   `json:"downloadable,omitempty"`
	CreatedDate         *string `json:"createdDate,omitempty"`
}

type AppStoreVersionRelationships struct {
	App                  *ResourceIdentifier `json:"app,omitempty"`
	AppStoreReviewDetail *ResourceIdentifier `json:"appStoreReviewDetail,omitempty"`
	Build                *ResourceIdentifier `json:"build,omitempty"`
}

type AppStoreVersionCreateRequest struct {
	Type          string                             `json:"type"`
	Attributes    AppStoreVersionCreateAttributes    `json:"attributes"`
	Relationships AppStoreVersionCreateRelationships `json:"relationships"`
}

type AppStoreVersionCreateAttributes struct {
	Platform            string  `json:"platform"`
	VersionString       string  `json:"versionString"`
	Copyright           *string `json:"copyright,omitempty"`
	ReviewType          *string `json:"reviewType,omitempty"`
	ReleaseType         *string `json:"releaseType,omitempty"`
	EarliestReleaseDate *string `json:"earliestReleaseDate,omitempty"`
	UsesIdfa            *bool   `json:"usesIdfa,omitempty"`
}

type AppStoreVersionCreateRelationships struct {
	App ResourceIdentifier `json:"app"`
}

type AppStoreVersionUpdateRequest struct {
	Type       string                          `json:"type"`
	ID         string                          `json:"id"`
	Attributes AppStoreVersionUpdateAttributes `json:"attributes"`
}

// AppStoreVersionUpdateAttributes omits platform: a version belongs to one
// platform for its whole life, and Apple's update request has no member for it.
type AppStoreVersionUpdateAttributes struct {
	VersionString       *string `json:"versionString,omitempty"`
	Copyright           *string `json:"copyright,omitempty"`
	ReviewType          *string `json:"reviewType,omitempty"`
	ReleaseType         *string `json:"releaseType,omitempty"`
	EarliestReleaseDate *string `json:"earliestReleaseDate,omitempty"`
	UsesIdfa            *bool   `json:"usesIdfa,omitempty"`
}

// AppStoreVersionLocalization is the per-release, per-language product page
// text.
//
// keywords is a single comma-separated string capped at 100 characters, not a
// list -- Apple has never modelled it as one.
type AppStoreVersionLocalization struct {
	Type          string                                    `json:"type"`
	ID            string                                    `json:"id"`
	Attributes    AppStoreVersionLocalizationAttributes     `json:"attributes"`
	Relationships *AppStoreVersionLocalizationRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                            `json:"links,omitempty"`
}

type AppStoreVersionLocalizationAttributes struct {
	Locale          string  `json:"locale,omitempty"`
	Description     *string `json:"description,omitempty"`
	Keywords        *string `json:"keywords,omitempty"`
	PromotionalText *string `json:"promotionalText,omitempty"`
	MarketingURL    *string `json:"marketingUrl,omitempty"`
	SupportURL      *string `json:"supportUrl,omitempty"`
	WhatsNew        *string `json:"whatsNew,omitempty"`
}

type AppStoreVersionLocalizationRelationships struct {
	AppStoreVersion *ResourceIdentifier `json:"appStoreVersion,omitempty"`
}

type AppStoreVersionLocalizationCreateRequest struct {
	Type          string                                         `json:"type"`
	Attributes    AppStoreVersionLocalizationCreateAttributes    `json:"attributes"`
	Relationships AppStoreVersionLocalizationCreateRelationships `json:"relationships"`
}

type AppStoreVersionLocalizationCreateAttributes struct {
	Locale          string  `json:"locale"`
	Description     *string `json:"description,omitempty"`
	Keywords        *string `json:"keywords,omitempty"`
	PromotionalText *string `json:"promotionalText,omitempty"`
	MarketingURL    *string `json:"marketingUrl,omitempty"`
	SupportURL      *string `json:"supportUrl,omitempty"`
	WhatsNew        *string `json:"whatsNew,omitempty"`
}

type AppStoreVersionLocalizationCreateRelationships struct {
	AppStoreVersion ResourceIdentifier `json:"appStoreVersion"`
}

type AppStoreVersionLocalizationUpdateRequest struct {
	Type       string                                      `json:"type"`
	ID         string                                      `json:"id"`
	Attributes AppStoreVersionLocalizationUpdateAttributes `json:"attributes"`
}

// AppStoreVersionLocalizationUpdateAttributes omits locale, which identifies
// the record.
type AppStoreVersionLocalizationUpdateAttributes struct {
	Description     *string `json:"description,omitempty"`
	Keywords        *string `json:"keywords,omitempty"`
	PromotionalText *string `json:"promotionalText,omitempty"`
	MarketingURL    *string `json:"marketingUrl,omitempty"`
	SupportURL      *string `json:"supportUrl,omitempty"`
	WhatsNew        *string `json:"whatsNew,omitempty"`
}

// AppStoreReviewDetail is what App Review reads before testing a build: who to
// contact, how to sign in, and anything they need to be told.
//
// A version has exactly one. Apple publishes no DELETE for it.
type AppStoreReviewDetail struct {
	Type          string                             `json:"type"`
	ID            string                             `json:"id"`
	Attributes    AppStoreReviewDetailAttributes     `json:"attributes"`
	Relationships *AppStoreReviewDetailRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                     `json:"links,omitempty"`
}

type AppStoreReviewDetailAttributes struct {
	ContactFirstName    *string `json:"contactFirstName,omitempty"`
	ContactLastName     *string `json:"contactLastName,omitempty"`
	ContactPhone        *string `json:"contactPhone,omitempty"`
	ContactEmail        *string `json:"contactEmail,omitempty"`
	DemoAccountName     *string `json:"demoAccountName,omitempty"`
	DemoAccountPassword *string `json:"demoAccountPassword,omitempty"`
	DemoAccountRequired *bool   `json:"demoAccountRequired,omitempty"`
	Notes               *string `json:"notes,omitempty"`
}

type AppStoreReviewDetailRelationships struct {
	AppStoreVersion *ResourceIdentifier `json:"appStoreVersion,omitempty"`
}

type AppStoreReviewDetailCreateRequest struct {
	Type          string                                  `json:"type"`
	Attributes    AppStoreReviewDetailAttributes          `json:"attributes"`
	Relationships AppStoreReviewDetailCreateRelationships `json:"relationships"`
}

type AppStoreReviewDetailCreateRelationships struct {
	AppStoreVersion ResourceIdentifier `json:"appStoreVersion"`
}

type AppStoreReviewDetailUpdateRequest struct {
	Type       string                         `json:"type"`
	ID         string                         `json:"id"`
	Attributes AppStoreReviewDetailAttributes `json:"attributes"`
}
