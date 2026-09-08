// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Subscription state enumeration.
//
// Every value is Apple's; the provider never sends one. A subscription starts
// at MISSING_METADATA and only leaves it once it has a localization and a
// price, which is why those are separate resources rather than optional
// attributes -- a subscription on its own is always incomplete.
type SubscriptionState string

const (
	SubscriptionStateMissingMetadata          SubscriptionState = "MISSING_METADATA"
	SubscriptionStateReadyToSubmit            SubscriptionState = "READY_TO_SUBMIT"
	SubscriptionStateWaitingForReview         SubscriptionState = "WAITING_FOR_REVIEW"
	SubscriptionStateInReview                 SubscriptionState = "IN_REVIEW"
	SubscriptionStateDeveloperActionNeeded    SubscriptionState = "DEVELOPER_ACTION_NEEDED"
	SubscriptionStatePendingBinaryApproval    SubscriptionState = "PENDING_BINARY_APPROVAL"
	SubscriptionStateApproved                 SubscriptionState = "APPROVED"
	SubscriptionStateDeveloperRemovedFromSale SubscriptionState = "DEVELOPER_REMOVED_FROM_SALE"
	SubscriptionStateRemovedFromSale          SubscriptionState = "REMOVED_FROM_SALE"
	SubscriptionStateRejected                 SubscriptionState = "REJECTED"
)

// Subscription period enumeration.
//
// These are the only durations the App Store offers for an auto-renewable
// subscription. Apple's create request documents subscriptionPeriod as a bare
// string, but the update request enumerates it, and both accept the same set.
type SubscriptionPeriod string

const (
	SubscriptionPeriodOneWeek     SubscriptionPeriod = "ONE_WEEK"
	SubscriptionPeriodOneMonth    SubscriptionPeriod = "ONE_MONTH"
	SubscriptionPeriodTwoMonths   SubscriptionPeriod = "TWO_MONTHS"
	SubscriptionPeriodThreeMonths SubscriptionPeriod = "THREE_MONTHS"
	SubscriptionPeriodSixMonths   SubscriptionPeriod = "SIX_MONTHS"
	SubscriptionPeriodOneYear     SubscriptionPeriod = "ONE_YEAR"
)

// ValidSubscriptionPeriods lists the accepted subscription durations.
var ValidSubscriptionPeriods = []string{
	string(SubscriptionPeriodOneWeek),
	string(SubscriptionPeriodOneMonth),
	string(SubscriptionPeriodTwoMonths),
	string(SubscriptionPeriodThreeMonths),
	string(SubscriptionPeriodSixMonths),
	string(SubscriptionPeriodOneYear),
}

// Subscription localization state enumeration.
type SubscriptionLocalizationState string

const (
	SubscriptionLocalizationStatePrepareForSubmission SubscriptionLocalizationState = "PREPARE_FOR_SUBMISSION"
	SubscriptionLocalizationStateWaitingForReview     SubscriptionLocalizationState = "WAITING_FOR_REVIEW"
	SubscriptionLocalizationStateApproved             SubscriptionLocalizationState = "APPROVED"
	SubscriptionLocalizationStateRejected             SubscriptionLocalizationState = "REJECTED"
)

// Subscription plan type enumeration.
//
// MONTHLY is the recurring price of the subscription. UPFRONT is the price of
// a pre-paid plan, where the customer pays the whole term in advance.
type SubscriptionPlanType string

const (
	SubscriptionPlanTypeMonthly SubscriptionPlanType = "MONTHLY"
	SubscriptionPlanTypeUpfront SubscriptionPlanType = "UPFRONT"
)

// ValidSubscriptionPlanTypes lists the accepted plan types.
var ValidSubscriptionPlanTypes = []string{
	string(SubscriptionPlanTypeMonthly),
	string(SubscriptionPlanTypeUpfront),
}

// --- Subscription groups ---

// SubscriptionGroup resource model.
type SubscriptionGroup struct {
	Type       string                      `json:"type"`
	ID         string                      `json:"id"`
	Attributes SubscriptionGroupAttributes `json:"attributes"`
	Links      *ResourceLinks              `json:"links,omitempty"`
}

// SubscriptionGroupAttributes holds the group's only attribute.
//
// referenceName is internal to App Store Connect. What customers see is a
// subscriptionGroupLocalization, which this provider does not manage.
type SubscriptionGroupAttributes struct {
	ReferenceName string `json:"referenceName"`
}

type SubscriptionGroupCreateRequest struct {
	Type          string                               `json:"type"`
	Attributes    SubscriptionGroupAttributes          `json:"attributes"`
	Relationships SubscriptionGroupCreateRelationships `json:"relationships"`
}

// SubscriptionGroupCreateRelationships names the app that owns the group.
//
// There is no matching field on the update request and no "app" value for the
// include parameter on GET, so the owning app is write-once and unreadable
// afterwards -- which is why the resource keeps the configured value and import
// takes the composite "<app_id>/<group_id>" form.
type SubscriptionGroupCreateRelationships struct {
	App ResourceIdentifier `json:"app"`
}

type SubscriptionGroupUpdateRequest struct {
	Type       string                      `json:"type"`
	ID         string                      `json:"id"`
	Attributes SubscriptionGroupAttributes `json:"attributes"`
}

// --- Subscriptions ---

// Subscription resource model.
type Subscription struct {
	Type          string                     `json:"type"`
	ID            string                     `json:"id"`
	Attributes    SubscriptionAttributes     `json:"attributes"`
	Relationships *SubscriptionRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks             `json:"links,omitempty"`
}

// SubscriptionAttributes covers both what is sent and what is returned.
type SubscriptionAttributes struct {
	Name               string              `json:"name,omitempty"`
	ProductID          string              `json:"productId,omitempty"`
	FamilySharable     *bool               `json:"familySharable,omitempty"`
	State              *SubscriptionState  `json:"state,omitempty"`
	SubscriptionPeriod *SubscriptionPeriod `json:"subscriptionPeriod,omitempty"`
	ReviewNote         *string             `json:"reviewNote,omitempty"`
	GroupLevel         *int                `json:"groupLevel,omitempty"`
}

// SubscriptionRelationships holds the group linkage.
//
// Apple populates the "data" member only when the request asks for it with
// include=group; otherwise the relationship carries links alone.
type SubscriptionRelationships struct {
	Group *ResourceIdentifier `json:"group,omitempty"`
}

type SubscriptionCreateRequest struct {
	Type          string                          `json:"type"`
	Attributes    SubscriptionCreateAttributes    `json:"attributes"`
	Relationships SubscriptionCreateRelationships `json:"relationships"`
}

// SubscriptionCreateAttributes are the attributes accepted at creation.
//
// name and productId are the only required ones. Apple assigns groupLevel 1
// when it is omitted.
type SubscriptionCreateAttributes struct {
	Name               string              `json:"name"`
	ProductID          string              `json:"productId"`
	FamilySharable     *bool               `json:"familySharable,omitempty"`
	SubscriptionPeriod *SubscriptionPeriod `json:"subscriptionPeriod,omitempty"`
	ReviewNote         *string             `json:"reviewNote,omitempty"`
	GroupLevel         *int                `json:"groupLevel,omitempty"`
}

type SubscriptionCreateRelationships struct {
	Group ResourceIdentifier `json:"group"`
}

type SubscriptionUpdateRequest struct {
	Type       string                       `json:"type"`
	ID         string                       `json:"id"`
	Attributes SubscriptionUpdateAttributes `json:"attributes"`
}

// SubscriptionUpdateAttributes omits productId deliberately.
//
// Apple's SubscriptionUpdateRequest has no productId member: the product
// identifier is immutable once the subscription exists, and a change to it has
// to replace the resource. The group is likewise absent, so a subscription
// cannot be moved between groups either.
type SubscriptionUpdateAttributes struct {
	Name               *string             `json:"name,omitempty"`
	FamilySharable     *bool               `json:"familySharable,omitempty"`
	SubscriptionPeriod *SubscriptionPeriod `json:"subscriptionPeriod,omitempty"`
	ReviewNote         *string             `json:"reviewNote,omitempty"`
	GroupLevel         *int                `json:"groupLevel,omitempty"`
}

// --- Subscription localizations ---

// SubscriptionLocalization resource model.
type SubscriptionLocalization struct {
	Type          string                                 `json:"type"`
	ID            string                                 `json:"id"`
	Attributes    SubscriptionLocalizationAttributes     `json:"attributes"`
	Relationships *SubscriptionLocalizationRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                         `json:"links,omitempty"`
}

type SubscriptionLocalizationAttributes struct {
	Name        string                         `json:"name,omitempty"`
	Description *string                        `json:"description,omitempty"`
	Locale      string                         `json:"locale,omitempty"`
	State       *SubscriptionLocalizationState `json:"state,omitempty"`
}

type SubscriptionLocalizationRelationships struct {
	Subscription *ResourceIdentifier `json:"subscription,omitempty"`
}

type SubscriptionLocalizationCreateRequest struct {
	Type          string                                      `json:"type"`
	Attributes    SubscriptionLocalizationCreateAttributes    `json:"attributes"`
	Relationships SubscriptionLocalizationCreateRelationships `json:"relationships"`
}

type SubscriptionLocalizationCreateAttributes struct {
	Name        string  `json:"name"`
	Locale      string  `json:"locale"`
	Description *string `json:"description,omitempty"`
}

type SubscriptionLocalizationCreateRelationships struct {
	Subscription ResourceIdentifier `json:"subscription"`
}

type SubscriptionLocalizationUpdateRequest struct {
	Type       string                                   `json:"type"`
	ID         string                                   `json:"id"`
	Attributes SubscriptionLocalizationUpdateAttributes `json:"attributes"`
}

// SubscriptionLocalizationUpdateAttributes omits locale: it identifies the
// localization and cannot be changed in place.
type SubscriptionLocalizationUpdateAttributes struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// --- Subscription prices ---

// SubscriptionPrice resource model.
type SubscriptionPrice struct {
	Type          string                          `json:"type"`
	ID            string                          `json:"id"`
	Attributes    SubscriptionPriceAttributes     `json:"attributes"`
	Relationships *SubscriptionPriceRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                  `json:"links,omitempty"`
}

// SubscriptionPriceAttributes reports the scheduled price.
//
// startDate is a plain date (YYYY-MM-DD), not a timestamp, and is null for a
// price that is already in effect. "preserved" reports whether existing
// subscribers were held at their previous price, which is the read side of the
// preserveCurrentPrice flag sent at creation.
type SubscriptionPriceAttributes struct {
	StartDate *string               `json:"startDate,omitempty"`
	Preserved *bool                 `json:"preserved,omitempty"`
	PlanType  *SubscriptionPlanType `json:"planType,omitempty"`
}

type SubscriptionPriceRelationships struct {
	Territory              *ResourceIdentifier `json:"territory,omitempty"`
	SubscriptionPricePoint *ResourceIdentifier `json:"subscriptionPricePoint,omitempty"`
}

// SubscriptionPriceCreateRequest has no update counterpart.
//
// Apple publishes no PATCH for subscriptionPrices: a price change is a new
// price record, and the old one is deleted. Every attribute of the Terraform
// resource therefore forces replacement.
type SubscriptionPriceCreateRequest struct {
	Type          string                               `json:"type"`
	Attributes    *SubscriptionPriceCreateAttributes   `json:"attributes,omitempty"`
	Relationships SubscriptionPriceCreateRelationships `json:"relationships"`
}

type SubscriptionPriceCreateAttributes struct {
	StartDate            *string               `json:"startDate,omitempty"`
	PreserveCurrentPrice *bool                 `json:"preserveCurrentPrice,omitempty"`
	PlanType             *SubscriptionPlanType `json:"planType,omitempty"`
}

// SubscriptionPriceCreateRelationships names the subscription and the price
// point. The territory is optional because a price point already belongs to
// one; sending it is only meaningful when equalizing across territories.
type SubscriptionPriceCreateRelationships struct {
	Subscription           ResourceIdentifier  `json:"subscription"`
	SubscriptionPricePoint ResourceIdentifier  `json:"subscriptionPricePoint"`
	Territory              *ResourceIdentifier `json:"territory,omitempty"`
}

// --- Subscription price points ---

// SubscriptionPricePoint is Apple's catalogue of permitted prices.
//
// A price is never expressed as a number: it is a reference to one of these,
// each of which fixes a customer price and the developer proceeds for one
// territory. They are read-only and are looked up through the
// apple_subscription_price_points data source.
type SubscriptionPricePoint struct {
	Type          string                               `json:"type"`
	ID            string                               `json:"id"`
	Attributes    SubscriptionPricePointAttributes     `json:"attributes"`
	Relationships *SubscriptionPricePointRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                       `json:"links,omitempty"`
}

type SubscriptionPricePointAttributes struct {
	CustomerPrice string `json:"customerPrice,omitempty"`
	Proceeds      string `json:"proceeds,omitempty"`
	ProceedsYear2 string `json:"proceedsYear2,omitempty"`
}

type SubscriptionPricePointRelationships struct {
	Territory *ResourceIdentifier `json:"territory,omitempty"`
}
