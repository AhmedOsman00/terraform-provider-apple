// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// BetaGroup is one TestFlight tester group.
//
// Apple models internal and external groups as the same record distinguished by
// isInternalGroup, but the two are not interchangeable: an internal group draws
// its members from the team's App Store Connect users and needs no beta review,
// while an external group takes arbitrary email addresses, can be opened to a
// public link, and cannot receive a build until App Review has approved one.
// Neither isInternalGroup nor hasAccessToAllBuilds appears in Apple's update
// request, so both are fixed for the life of the group.
type BetaGroup struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id"`
	Attributes    BetaGroupAttributes     `json:"attributes"`
	Relationships *BetaGroupRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks          `json:"links,omitempty"`
}

// BetaGroupAttributes reports what Apple holds about a group.
//
// publicLink and publicLinkId are Apple's, issued when publicLinkEnabled is
// turned on and absent entirely from an internal group.
type BetaGroupAttributes struct {
	Name                                 *string `json:"name,omitempty"`
	CreatedDate                          *string `json:"createdDate,omitempty"`
	IsInternalGroup                      *bool   `json:"isInternalGroup,omitempty"`
	HasAccessToAllBuilds                 *bool   `json:"hasAccessToAllBuilds,omitempty"`
	PublicLinkEnabled                    *bool   `json:"publicLinkEnabled,omitempty"`
	PublicLinkID                         *string `json:"publicLinkId,omitempty"`
	PublicLink                           *string `json:"publicLink,omitempty"`
	PublicLinkLimitEnabled               *bool   `json:"publicLinkLimitEnabled,omitempty"`
	PublicLinkLimit                      *int64  `json:"publicLinkLimit,omitempty"`
	FeedbackEnabled                      *bool   `json:"feedbackEnabled,omitempty"`
	IOSBuildsAvailableForAppleSiliconMac *bool   `json:"iosBuildsAvailableForAppleSiliconMac,omitempty"`
	IOSBuildsAvailableForAppleVision     *bool   `json:"iosBuildsAvailableForAppleVision,omitempty"`
}

type BetaGroupRelationships struct {
	App *ResourceIdentifier `json:"app,omitempty"`
}

type BetaGroupCreateRequest struct {
	Type          string                       `json:"type"`
	Attributes    BetaGroupCreateAttributes    `json:"attributes"`
	Relationships BetaGroupCreateRelationships `json:"relationships"`
}

// BetaGroupCreateAttributes is the only place isInternalGroup and
// hasAccessToAllBuilds can be set. Apple's update request carries neither.
type BetaGroupCreateAttributes struct {
	Name                                 string `json:"name"`
	IsInternalGroup                      *bool  `json:"isInternalGroup,omitempty"`
	HasAccessToAllBuilds                 *bool  `json:"hasAccessToAllBuilds,omitempty"`
	PublicLinkEnabled                    *bool  `json:"publicLinkEnabled,omitempty"`
	PublicLinkLimitEnabled               *bool  `json:"publicLinkLimitEnabled,omitempty"`
	PublicLinkLimit                      *int64 `json:"publicLinkLimit,omitempty"`
	FeedbackEnabled                      *bool  `json:"feedbackEnabled,omitempty"`
	IOSBuildsAvailableForAppleSiliconMac *bool  `json:"iosBuildsAvailableForAppleSiliconMac,omitempty"`
	IOSBuildsAvailableForAppleVision     *bool  `json:"iosBuildsAvailableForAppleVision,omitempty"`
}

type BetaGroupCreateRelationships struct {
	App ResourceIdentifier `json:"app"`
}

type BetaGroupUpdateRequest struct {
	Type       string                    `json:"type"`
	ID         string                    `json:"id"`
	Attributes BetaGroupUpdateAttributes `json:"attributes"`
}

// BetaGroupUpdateAttributes omits isInternalGroup and hasAccessToAllBuilds: a
// group does not change kind, and Apple has no member for either here.
type BetaGroupUpdateAttributes struct {
	Name                                 *string `json:"name,omitempty"`
	PublicLinkEnabled                    *bool   `json:"publicLinkEnabled,omitempty"`
	PublicLinkLimitEnabled               *bool   `json:"publicLinkLimitEnabled,omitempty"`
	PublicLinkLimit                      *int64  `json:"publicLinkLimit,omitempty"`
	FeedbackEnabled                      *bool   `json:"feedbackEnabled,omitempty"`
	IOSBuildsAvailableForAppleSiliconMac *bool   `json:"iosBuildsAvailableForAppleSiliconMac,omitempty"`
	IOSBuildsAvailableForAppleVision     *bool   `json:"iosBuildsAvailableForAppleVision,omitempty"`
}

// BetaAppLocalization is the TestFlight "What to Test" header text for one
// language: the description testers read on the app's TestFlight page, plus the
// feedback address and the marketing and privacy links shown beside it.
//
// It hangs off the app and survives every build, which is what separates it
// from BetaBuildLocalization.
type BetaAppLocalization struct {
	Type          string                            `json:"type"`
	ID            string                            `json:"id"`
	Attributes    BetaAppLocalizationAttributes     `json:"attributes"`
	Relationships *BetaAppLocalizationRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                    `json:"links,omitempty"`
}

type BetaAppLocalizationAttributes struct {
	Locale            string  `json:"locale,omitempty"`
	Description       *string `json:"description,omitempty"`
	FeedbackEmail     *string `json:"feedbackEmail,omitempty"`
	MarketingURL      *string `json:"marketingUrl,omitempty"`
	PrivacyPolicyURL  *string `json:"privacyPolicyUrl,omitempty"`
	TVOSPrivacyPolicy *string `json:"tvOsPrivacyPolicy,omitempty"`
}

type BetaAppLocalizationRelationships struct {
	App *ResourceIdentifier `json:"app,omitempty"`
}

type BetaAppLocalizationCreateRequest struct {
	Type          string                                 `json:"type"`
	Attributes    BetaAppLocalizationCreateAttributes    `json:"attributes"`
	Relationships BetaAppLocalizationCreateRelationships `json:"relationships"`
}

type BetaAppLocalizationCreateAttributes struct {
	Locale            string  `json:"locale"`
	Description       *string `json:"description,omitempty"`
	FeedbackEmail     *string `json:"feedbackEmail,omitempty"`
	MarketingURL      *string `json:"marketingUrl,omitempty"`
	PrivacyPolicyURL  *string `json:"privacyPolicyUrl,omitempty"`
	TVOSPrivacyPolicy *string `json:"tvOsPrivacyPolicy,omitempty"`
}

type BetaAppLocalizationCreateRelationships struct {
	App ResourceIdentifier `json:"app"`
}

type BetaAppLocalizationUpdateRequest struct {
	Type       string                              `json:"type"`
	ID         string                              `json:"id"`
	Attributes BetaAppLocalizationUpdateAttributes `json:"attributes"`
}

// BetaAppLocalizationUpdateAttributes omits locale, which identifies the record.
type BetaAppLocalizationUpdateAttributes struct {
	Description       *string `json:"description,omitempty"`
	FeedbackEmail     *string `json:"feedbackEmail,omitempty"`
	MarketingURL      *string `json:"marketingUrl,omitempty"`
	PrivacyPolicyURL  *string `json:"privacyPolicyUrl,omitempty"`
	TVOSPrivacyPolicy *string `json:"tvOsPrivacyPolicy,omitempty"`
}

// BetaBuildLocalization is the per-build "What to Test" note in one language.
//
// It hangs off a build rather than the app, so it is replaced with every
// upload -- the opposite lifetime from BetaAppLocalization, and the same split
// AppInfo and AppStoreVersion have on the App Store side.
type BetaBuildLocalization struct {
	Type          string                              `json:"type"`
	ID            string                              `json:"id"`
	Attributes    BetaBuildLocalizationAttributes     `json:"attributes"`
	Relationships *BetaBuildLocalizationRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                      `json:"links,omitempty"`
}

type BetaBuildLocalizationAttributes struct {
	Locale   string  `json:"locale,omitempty"`
	WhatsNew *string `json:"whatsNew,omitempty"`
}

type BetaBuildLocalizationRelationships struct {
	Build *ResourceIdentifier `json:"build,omitempty"`
}

type BetaBuildLocalizationCreateRequest struct {
	Type          string                                   `json:"type"`
	Attributes    BetaBuildLocalizationCreateAttributes    `json:"attributes"`
	Relationships BetaBuildLocalizationCreateRelationships `json:"relationships"`
}

type BetaBuildLocalizationCreateAttributes struct {
	Locale   string  `json:"locale"`
	WhatsNew *string `json:"whatsNew,omitempty"`
}

type BetaBuildLocalizationCreateRelationships struct {
	Build ResourceIdentifier `json:"build"`
}

type BetaBuildLocalizationUpdateRequest struct {
	Type       string                                `json:"type"`
	ID         string                                `json:"id"`
	Attributes BetaBuildLocalizationUpdateAttributes `json:"attributes"`
}

// BetaBuildLocalizationUpdateAttributes carries whatsNew alone: locale
// identifies the record and the build is fixed.
type BetaBuildLocalizationUpdateAttributes struct {
	WhatsNew *string `json:"whatsNew,omitempty"`
}

// BetaAppReviewDetail is what the TestFlight beta reviewers are told before
// they test a build bound for an external group.
//
// It is the beta-review counterpart of AppStoreReviewDetail and a different
// Apple record from it: one per app rather than one per version, and it gates
// external distribution rather than the App Store listing. Apple publishes no
// POST for it -- the record is created with the app -- so it is adopted rather
// than created, the way apple_app_settings is.
type BetaAppReviewDetail struct {
	Type          string                            `json:"type"`
	ID            string                            `json:"id"`
	Attributes    BetaAppReviewDetailAttributes     `json:"attributes"`
	Relationships *BetaAppReviewDetailRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                    `json:"links,omitempty"`
}

type BetaAppReviewDetailAttributes struct {
	ContactFirstName    *string `json:"contactFirstName,omitempty"`
	ContactLastName     *string `json:"contactLastName,omitempty"`
	ContactPhone        *string `json:"contactPhone,omitempty"`
	ContactEmail        *string `json:"contactEmail,omitempty"`
	DemoAccountName     *string `json:"demoAccountName,omitempty"`
	DemoAccountPassword *string `json:"demoAccountPassword,omitempty"`
	DemoAccountRequired *bool   `json:"demoAccountRequired,omitempty"`
	Notes               *string `json:"notes,omitempty"`
}

type BetaAppReviewDetailRelationships struct {
	App *ResourceIdentifier `json:"app,omitempty"`
}

type BetaAppReviewDetailUpdateRequest struct {
	Type       string                        `json:"type"`
	ID         string                        `json:"id"`
	Attributes BetaAppReviewDetailAttributes `json:"attributes"`
}

// BetaTester is one person who can install TestFlight builds.
//
// The record is the team's, not the app's: a tester is identified by email
// address across the whole App Store Connect account, and reaches an app by
// belonging to one of its beta groups. Apple publishes no PATCH for it, so
// every attribute here is fixed once the record exists -- a tester who already
// exists keeps the name they were created with, whatever a later request sends.
type BetaTester struct {
	Type          string                   `json:"type"`
	ID            string                   `json:"id"`
	Attributes    BetaTesterAttributes     `json:"attributes"`
	Relationships *BetaTesterRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks           `json:"links,omitempty"`
}

// BetaTesterAttributes reports what Apple holds about a tester.
//
// inviteType is Apple's: EMAIL for a tester added by address, PUBLIC_LINK for
// one who joined through a group's public TestFlight link.
type BetaTesterAttributes struct {
	FirstName  *string `json:"firstName,omitempty"`
	LastName   *string `json:"lastName,omitempty"`
	Email      *string `json:"email,omitempty"`
	InviteType *string `json:"inviteType,omitempty"`
}

type BetaTesterRelationships struct {
	Apps       *ResourceIdentifiers `json:"apps,omitempty"`
	BetaGroups *ResourceIdentifiers `json:"betaGroups,omitempty"`
	Builds     *ResourceIdentifiers `json:"builds,omitempty"`
}

type BetaTesterCreateRequest struct {
	Type          string                        `json:"type"`
	Attributes    BetaTesterCreateAttributes    `json:"attributes"`
	Relationships BetaTesterCreateRelationships `json:"relationships"`
}

// BetaTesterCreateAttributes is the only place a tester's name can be set.
// Apple has no update request for the record at all.
type BetaTesterCreateAttributes struct {
	Email     string  `json:"email"`
	FirstName *string `json:"firstName,omitempty"`
	LastName  *string `json:"lastName,omitempty"`
}

// BetaTesterCreateRelationships carries the groups the new tester joins.
//
// Creating a tester with no group leaves them attached to nothing, so this is
// how a tester reaches an app -- and sending a group here is what makes Apple
// issue the TestFlight invitation.
type BetaTesterCreateRelationships struct {
	BetaGroups ResourceIdentifiers `json:"betaGroups"`
}

// BetaTesterLinkageRequest is the body of the group membership endpoints:
// POST adds the listed testers to a group, DELETE removes them from it.
//
// Removing a linkage is not the same as deleting the tester. The record stays
// in the account's tester list and in every other group it belongs to, which is
// why this provider removes membership rather than calling
// DELETE /v1/betaTesters/{id}.
type BetaTesterLinkageRequest struct {
	Data []ResourceData `json:"data"`
}
