// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// App resource model.
//
// An app record cannot be created or deleted through the API: Apple's
// documentation for the POST endpoints states "Don't use this API to create new
// apps; instead, create new apps on the App Store Connect website." It can,
// however, be updated -- PATCH /v1/apps carries the handful of account-level
// settings modelled below, of which contentRightsDeclaration is the one App
// Store Connect blocks a submission on.
//
// Everything a customer reads lives elsewhere: the name and subtitle on an
// AppInfoLocalization, the description and keywords on an
// AppStoreVersionLocalization, the categories on the AppInfo itself.
type App struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Attributes AppAttributes  `json:"attributes"`
	Links      *ResourceLinks `json:"links,omitempty"`
}

// App attributes.
//
// The identifying fields (name, bundleId, sku) are read-only in practice: name
// is set by the App Store Connect website and sku never changes. The rest are
// what PATCH /v1/apps accepts.
type AppAttributes struct {
	Name                                   string  `json:"name,omitempty"`
	BundleID                               string  `json:"bundleId,omitempty"`
	SKU                                    string  `json:"sku,omitempty"`
	PrimaryLocale                          string  `json:"primaryLocale,omitempty"`
	ContentRightsDeclaration               *string `json:"contentRightsDeclaration,omitempty"`
	AccessibilityURL                       *string `json:"accessibilityUrl,omitempty"`
	SubscriptionStatusURL                  *string `json:"subscriptionStatusUrl,omitempty"`
	SubscriptionStatusURLVersion           *string `json:"subscriptionStatusUrlVersion,omitempty"`
	SubscriptionStatusURLForSandbox        *string `json:"subscriptionStatusUrlForSandbox,omitempty"`
	SubscriptionStatusURLVersionForSandbox *string `json:"subscriptionStatusUrlVersionForSandbox,omitempty"`
	StreamlinedPurchasingEnabled           *bool   `json:"streamlinedPurchasingEnabled,omitempty"`
	IsOrEverWasMadeForKids                 *bool   `json:"isOrEverWasMadeForKids,omitempty"`
}

// Content rights declaration enumeration.
//
// This is App Store Connect's "Content Rights Information" question, and a
// submission is blocked until it is answered. Apple offers exactly two answers:
// either the app contains third-party content the developer has the rights to
// use, or it contains none.
const (
	ContentRightsDoesNotUseThirdPartyContent = "DOES_NOT_USE_THIRD_PARTY_CONTENT"
	ContentRightsUsesThirdPartyContent       = "USES_THIRD_PARTY_CONTENT"
)

// ValidContentRightsDeclarations lists the accepted content rights answers.
var ValidContentRightsDeclarations = []string{
	ContentRightsDoesNotUseThirdPartyContent,
	ContentRightsUsesThirdPartyContent,
}

// ValidSubscriptionStatusURLVersions lists the accepted server notification
// versions.
var ValidSubscriptionStatusURLVersions = []string{"V1", "V2"}

// AppUpdateRequest patches the app record.
//
// Only the attributes Apple accepts on PATCH /v1/apps appear here. name, sku
// and the bundle ID relationship are deliberately absent: the first two are not
// updatable at all and the third would repoint a live app at a different App
// ID, which is not something a Terraform attribute should be able to do by
// accident.
type AppUpdateRequest struct {
	Type       string              `json:"type"`
	ID         string              `json:"id"`
	Attributes AppUpdateAttributes `json:"attributes"`
}

type AppUpdateAttributes struct {
	PrimaryLocale                          *string `json:"primaryLocale,omitempty"`
	ContentRightsDeclaration               *string `json:"contentRightsDeclaration,omitempty"`
	AccessibilityURL                       *string `json:"accessibilityUrl,omitempty"`
	SubscriptionStatusURL                  *string `json:"subscriptionStatusUrl,omitempty"`
	SubscriptionStatusURLVersion           *string `json:"subscriptionStatusUrlVersion,omitempty"`
	SubscriptionStatusURLForSandbox        *string `json:"subscriptionStatusUrlForSandbox,omitempty"`
	SubscriptionStatusURLVersionForSandbox *string `json:"subscriptionStatusUrlVersionForSandbox,omitempty"`
	StreamlinedPurchasingEnabled           *bool   `json:"streamlinedPurchasingEnabled,omitempty"`
}
