// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// In-app purchase type enumeration.
//
// This is the one-time purchase side of the App Store: a consumable is bought
// repeatedly, a non-consumable once and forever, and a non-renewing
// subscription is a fixed-term entitlement the customer has to buy again by
// hand. Auto-renewable subscriptions are not here -- they are a separate Apple
// resource, modelled by apple_subscription.
type InAppPurchaseType string

const (
	InAppPurchaseTypeConsumable              InAppPurchaseType = "CONSUMABLE"
	InAppPurchaseTypeNonConsumable           InAppPurchaseType = "NON_CONSUMABLE"
	InAppPurchaseTypeNonRenewingSubscription InAppPurchaseType = "NON_RENEWING_SUBSCRIPTION"
)

// ValidInAppPurchaseTypes lists the accepted in-app purchase types.
var ValidInAppPurchaseTypes = []string{
	string(InAppPurchaseTypeConsumable),
	string(InAppPurchaseTypeNonConsumable),
	string(InAppPurchaseTypeNonRenewingSubscription),
}

// In-app purchase state enumeration.
//
// Every value is Apple's; the provider never sends one. A newly created
// purchase reports MISSING_METADATA until it has a localization, a price and an
// availability, which is why those are separate resources.
type InAppPurchaseState string

const (
	InAppPurchaseStateMissingMetadata          InAppPurchaseState = "MISSING_METADATA"
	InAppPurchaseStateWaitingForUpload         InAppPurchaseState = "WAITING_FOR_UPLOAD"
	InAppPurchaseStateProcessingContent        InAppPurchaseState = "PROCESSING_CONTENT"
	InAppPurchaseStateReadyToSubmit            InAppPurchaseState = "READY_TO_SUBMIT"
	InAppPurchaseStateWaitingForReview         InAppPurchaseState = "WAITING_FOR_REVIEW"
	InAppPurchaseStateInReview                 InAppPurchaseState = "IN_REVIEW"
	InAppPurchaseStateDeveloperActionNeeded    InAppPurchaseState = "DEVELOPER_ACTION_NEEDED"
	InAppPurchaseStatePendingBinaryApproval    InAppPurchaseState = "PENDING_BINARY_APPROVAL"
	InAppPurchaseStateApproved                 InAppPurchaseState = "APPROVED"
	InAppPurchaseStateDeveloperRemovedFromSale InAppPurchaseState = "DEVELOPER_REMOVED_FROM_SALE"
	InAppPurchaseStateRemovedFromSale          InAppPurchaseState = "REMOVED_FROM_SALE"
	InAppPurchaseStateRejected                 InAppPurchaseState = "REJECTED"
)

// ValidInAppPurchaseStates lists the states Apple reports.
var ValidInAppPurchaseStates = []string{
	string(InAppPurchaseStateMissingMetadata),
	string(InAppPurchaseStateWaitingForUpload),
	string(InAppPurchaseStateProcessingContent),
	string(InAppPurchaseStateReadyToSubmit),
	string(InAppPurchaseStateWaitingForReview),
	string(InAppPurchaseStateInReview),
	string(InAppPurchaseStateDeveloperActionNeeded),
	string(InAppPurchaseStatePendingBinaryApproval),
	string(InAppPurchaseStateApproved),
	string(InAppPurchaseStateDeveloperRemovedFromSale),
	string(InAppPurchaseStateRemovedFromSale),
	string(InAppPurchaseStateRejected),
}

// In-app purchase version state enumeration.
//
// A version is the draft that carries localized metadata and review images
// through App Review. PREPARE_FOR_SUBMISSION, DEVELOPER_REJECTED and REJECTED
// are the states in which its localizations can still be edited.
type InAppPurchaseVersionState string

const (
	InAppPurchaseVersionStatePrepareForSubmission   InAppPurchaseVersionState = "PREPARE_FOR_SUBMISSION"
	InAppPurchaseVersionStateReadyForReview         InAppPurchaseVersionState = "READY_FOR_REVIEW"
	InAppPurchaseVersionStateWaitingForReview       InAppPurchaseVersionState = "WAITING_FOR_REVIEW"
	InAppPurchaseVersionStateInReview               InAppPurchaseVersionState = "IN_REVIEW"
	InAppPurchaseVersionStateAccepted               InAppPurchaseVersionState = "ACCEPTED"
	InAppPurchaseVersionStateApproved               InAppPurchaseVersionState = "APPROVED"
	InAppPurchaseVersionStateReplacedWithNewVersion InAppPurchaseVersionState = "REPLACED_WITH_NEW_VERSION"
	InAppPurchaseVersionStateRejected               InAppPurchaseVersionState = "REJECTED"
	InAppPurchaseVersionStateDeveloperRejected      InAppPurchaseVersionState = "DEVELOPER_REJECTED"
)

// --- In-app purchases ---

// InAppPurchase resource model, Apple's InAppPurchaseV2.
//
// The v1 in-app purchase resource is a different, older shape; everything here
// talks to /v2/inAppPurchases, which is the one Apple documents for new work.
type InAppPurchase struct {
	Type          string                      `json:"type"`
	ID            string                      `json:"id"`
	Attributes    InAppPurchaseAttributes     `json:"attributes"`
	Relationships *InAppPurchaseRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks              `json:"links,omitempty"`
}

// InAppPurchaseAttributes covers both what is sent and what is returned.
type InAppPurchaseAttributes struct {
	Name              string              `json:"name,omitempty"`
	ProductID         string              `json:"productId,omitempty"`
	InAppPurchaseType *InAppPurchaseType  `json:"inAppPurchaseType,omitempty"`
	State             *InAppPurchaseState `json:"state,omitempty"`
	ReviewNote        *string             `json:"reviewNote,omitempty"`
	FamilySharable    *bool               `json:"familySharable,omitempty"`
	ContentHosting    *bool               `json:"contentHosting,omitempty"`
}

// InAppPurchaseRelationships holds the linkages Apple reports.
//
// There is deliberately no app member: InAppPurchaseV2 has no app relationship
// at all, and no include value produces one. The owning app is write-once and
// unreadable, exactly as it is for a subscription group, which is why the
// resource keeps the configured value and import takes a composite form.
type InAppPurchaseRelationships struct {
	IAPPriceSchedule          *ResourceIdentifier `json:"iapPriceSchedule,omitempty"`
	InAppPurchaseAvailability *ResourceIdentifier `json:"inAppPurchaseAvailability,omitempty"`
}

type InAppPurchaseCreateRequest struct {
	Type          string                           `json:"type"`
	Attributes    InAppPurchaseCreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseCreateRelationships `json:"relationships"`
}

// InAppPurchaseCreateAttributes are the attributes accepted at creation.
//
// name, productId and inAppPurchaseType are all required. The type is absent
// from the update request: a consumable cannot become a non-consumable.
type InAppPurchaseCreateAttributes struct {
	Name              string            `json:"name"`
	ProductID         string            `json:"productId"`
	InAppPurchaseType InAppPurchaseType `json:"inAppPurchaseType"`
	FamilySharable    *bool             `json:"familySharable,omitempty"`
	ReviewNote        *string           `json:"reviewNote,omitempty"`
}

type InAppPurchaseCreateRelationships struct {
	App ResourceIdentifier `json:"app"`
}

type InAppPurchaseUpdateRequest struct {
	Type       string                        `json:"type"`
	ID         string                        `json:"id"`
	Attributes InAppPurchaseUpdateAttributes `json:"attributes"`
}

// InAppPurchaseUpdateAttributes omits productId and inAppPurchaseType
// deliberately: Apple's update request has neither member, so a change to
// either has to replace the resource.
type InAppPurchaseUpdateAttributes struct {
	Name           *string `json:"name,omitempty"`
	FamilySharable *bool   `json:"familySharable,omitempty"`
	ReviewNote     *string `json:"reviewNote,omitempty"`
}

// --- In-app purchase versions ---

// InAppPurchaseVersion is the draft that owns an in-app purchase's localized
// metadata.
//
// Apple moved localizations behind versions in App Store Connect API 4.4.1 and
// deprecated the v1 localization endpoints that hid them. A localization is
// created against a version, never directly against the purchase.
type InAppPurchaseVersion struct {
	Type          string                             `json:"type"`
	ID            string                             `json:"id"`
	Attributes    InAppPurchaseVersionAttributes     `json:"attributes"`
	Relationships *InAppPurchaseVersionRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                     `json:"links,omitempty"`
}

type InAppPurchaseVersionAttributes struct {
	Version *int                       `json:"version,omitempty"`
	State   *InAppPurchaseVersionState `json:"state,omitempty"`
}

type InAppPurchaseVersionRelationships struct {
	InAppPurchase *ResourceIdentifier `json:"inAppPurchase,omitempty"`
}

// InAppPurchaseVersionCreateRequest carries no attributes: a version is
// nothing but a pointer at the purchase whose metadata it captures.
type InAppPurchaseVersionCreateRequest struct {
	Type          string                                  `json:"type"`
	Relationships InAppPurchaseVersionCreateRelationships `json:"relationships"`
}

type InAppPurchaseVersionCreateRelationships struct {
	InAppPurchase ResourceIdentifier `json:"inAppPurchase"`
}

// --- In-app purchase localizations ---

// InAppPurchaseLocalization is Apple's InAppPurchaseLocalizationV2.
//
// It carries no state of its own -- the review state lives on the version that
// owns it, unlike a subscription localization, which reports its own.
type InAppPurchaseLocalization struct {
	Type          string                                  `json:"type"`
	ID            string                                  `json:"id"`
	Attributes    InAppPurchaseLocalizationAttributes     `json:"attributes"`
	Relationships *InAppPurchaseLocalizationRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                          `json:"links,omitempty"`
}

type InAppPurchaseLocalizationAttributes struct {
	Name        string  `json:"name,omitempty"`
	Locale      string  `json:"locale,omitempty"`
	Description *string `json:"description,omitempty"`
}

type InAppPurchaseLocalizationRelationships struct {
	Version *ResourceIdentifier `json:"version,omitempty"`
}

type InAppPurchaseLocalizationCreateRequest struct {
	Type          string                                       `json:"type"`
	Attributes    InAppPurchaseLocalizationCreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseLocalizationCreateRelationships `json:"relationships"`
}

type InAppPurchaseLocalizationCreateAttributes struct {
	Name        string  `json:"name"`
	Locale      string  `json:"locale"`
	Description *string `json:"description,omitempty"`
}

type InAppPurchaseLocalizationCreateRelationships struct {
	Version ResourceIdentifier `json:"version"`
}

type InAppPurchaseLocalizationUpdateRequest struct {
	Type       string                                    `json:"type"`
	ID         string                                    `json:"id"`
	Attributes InAppPurchaseLocalizationUpdateAttributes `json:"attributes"`
}

// InAppPurchaseLocalizationUpdateAttributes omits locale: it identifies the
// localization and cannot be changed in place.
type InAppPurchaseLocalizationUpdateAttributes struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// --- In-app purchase price schedules ---

// InAppPurchasePriceSchedule is the whole price configuration of one purchase.
//
// Unlike a subscription price, which is an individual record, a one-time
// purchase has exactly one schedule: a base territory whose price Apple
// equalizes into every other territory, plus the manual prices that override
// the equalized ones. The schedule carries no attributes at all -- everything
// it says is said through relationships.
type InAppPurchasePriceSchedule struct {
	Type          string                                   `json:"type"`
	ID            string                                   `json:"id"`
	Relationships *InAppPurchasePriceScheduleRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                           `json:"links,omitempty"`
}

type InAppPurchasePriceScheduleRelationships struct {
	BaseTerritory   *ResourceIdentifier  `json:"baseTerritory,omitempty"`
	ManualPrices    *ResourceIdentifiers `json:"manualPrices,omitempty"`
	AutomaticPrices *ResourceIdentifiers `json:"automaticPrices,omitempty"`
}

// InAppPurchasePriceScheduleCreateRequest is the one request in this provider
// that uses JSON:API's "included" member.
//
// The prices do not exist yet when the schedule is created, so they are sent
// inline: each one appears in "included" carrying a client-chosen placeholder
// ID, and the manualPrices relationship references those same placeholders.
// Apple substitutes real IDs when it commits the write.
type InAppPurchasePriceScheduleCreateRequest struct {
	Data     InAppPurchasePriceScheduleCreateData `json:"data"`
	Included []InAppPurchasePriceInlineCreate     `json:"included,omitempty"`
}

type InAppPurchasePriceScheduleCreateData struct {
	Type          string                                        `json:"type"`
	Relationships InAppPurchasePriceScheduleCreateRelationships `json:"relationships"`
}

// InAppPurchasePriceScheduleCreateRelationships names the purchase, the base
// territory and the inline prices.
//
// baseTerritory is required. Apple added that requirement after the endpoint
// shipped -- "A base territory is now required when adding or creating a price
// for an in-app purchase".
type InAppPurchasePriceScheduleCreateRelationships struct {
	InAppPurchase ResourceIdentifier  `json:"inAppPurchase"`
	BaseTerritory ResourceIdentifier  `json:"baseTerritory"`
	ManualPrices  ResourceIdentifiers `json:"manualPrices"`
}

// InAppPurchasePriceInlineCreate is one price inside the "included" array.
type InAppPurchasePriceInlineCreate struct {
	Type          string                                      `json:"type"`
	ID            string                                      `json:"id"`
	Attributes    *InAppPurchasePriceInlineCreateAttributes   `json:"attributes,omitempty"`
	Relationships InAppPurchasePriceInlineCreateRelationships `json:"relationships"`
}

// InAppPurchasePriceInlineCreateAttributes carries plain dates (YYYY-MM-DD),
// not timestamps. A null startDate means the price is in effect now.
type InAppPurchasePriceInlineCreateAttributes struct {
	StartDate *string `json:"startDate"`
	EndDate   *string `json:"endDate"`
}

// InAppPurchasePriceInlineCreateRelationships names the price point and repeats
// the purchase. Apple requires both on the inline object even though the
// schedule already names the purchase.
type InAppPurchasePriceInlineCreateRelationships struct {
	InAppPurchaseV2         ResourceIdentifier `json:"inAppPurchaseV2"`
	InAppPurchasePricePoint ResourceIdentifier `json:"inAppPurchasePricePoint"`
}

// InAppPurchasePrice is one scheduled price in a territory.
type InAppPurchasePrice struct {
	Type          string                           `json:"type"`
	ID            string                           `json:"id"`
	Attributes    InAppPurchasePriceAttributes     `json:"attributes"`
	Relationships *InAppPurchasePriceRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                   `json:"links,omitempty"`
}

// InAppPurchasePriceAttributes reports the scheduled window.
//
// "manual" distinguishes a price the developer set from one Apple equalized
// from the base territory. Only manual prices are the provider's to manage.
type InAppPurchasePriceAttributes struct {
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
	Manual    *bool   `json:"manual,omitempty"`
}

type InAppPurchasePriceRelationships struct {
	Territory               *ResourceIdentifier `json:"territory,omitempty"`
	InAppPurchasePricePoint *ResourceIdentifier `json:"inAppPurchasePricePoint,omitempty"`
}

// --- In-app purchase price points ---

// InAppPurchasePricePoint is Apple's catalogue of permitted prices.
//
// As with subscriptions, a price is never a number: it is a reference to one of
// these, each fixing a customer price and the developer proceeds for one
// territory. There is no proceedsYear2 here -- the reduced commission after a
// year applies to subscriptions, not one-time purchases.
type InAppPurchasePricePoint struct {
	Type          string                                `json:"type"`
	ID            string                                `json:"id"`
	Attributes    InAppPurchasePricePointAttributes     `json:"attributes"`
	Relationships *InAppPurchasePricePointRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                        `json:"links,omitempty"`
}

type InAppPurchasePricePointAttributes struct {
	CustomerPrice string `json:"customerPrice,omitempty"`
	Proceeds      string `json:"proceeds,omitempty"`
}

type InAppPurchasePricePointRelationships struct {
	Territory *ResourceIdentifier `json:"territory,omitempty"`
}

// --- In-app purchase availability ---

// InAppPurchaseAvailability is the set of territories a purchase sells in.
//
// Apple publishes no PATCH and no DELETE for it: changing availability means
// POSTing a new availability for the same purchase, which replaces the old one.
type InAppPurchaseAvailability struct {
	Type          string                                  `json:"type"`
	ID            string                                  `json:"id"`
	Attributes    InAppPurchaseAvailabilityAttributes     `json:"attributes"`
	Relationships *InAppPurchaseAvailabilityRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                          `json:"links,omitempty"`
}

// InAppPurchaseAvailabilityAttributes holds the one attribute Apple keeps
// outside the territory list: whether territories the App Store adds in future
// are opted into automatically.
type InAppPurchaseAvailabilityAttributes struct {
	AvailableInNewTerritories *bool `json:"availableInNewTerritories,omitempty"`
}

type InAppPurchaseAvailabilityRelationships struct {
	AvailableTerritories *ResourceIdentifiers `json:"availableTerritories,omitempty"`
}

type InAppPurchaseAvailabilityCreateRequest struct {
	Type          string                                       `json:"type"`
	Attributes    InAppPurchaseAvailabilityCreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseAvailabilityCreateRelationships `json:"relationships"`
}

// InAppPurchaseAvailabilityCreateAttributes takes availableInNewTerritories as
// a plain bool: Apple documents it as required, so it is always sent.
type InAppPurchaseAvailabilityCreateAttributes struct {
	AvailableInNewTerritories bool `json:"availableInNewTerritories"`
}

type InAppPurchaseAvailabilityCreateRelationships struct {
	InAppPurchase        ResourceIdentifier  `json:"inAppPurchase"`
	AvailableTerritories ResourceIdentifiers `json:"availableTerritories"`
}
