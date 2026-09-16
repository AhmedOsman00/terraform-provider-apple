// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// AppPriceSchedule is the whole price configuration of an app.
//
// Apple retired price tiers: an app's price is a base territory whose price is
// equalized into every other storefront, plus the manual prices that override
// the equalized ones. The schedule carries no attributes at all -- everything
// it says is said through relationships -- and it is the same shape as an
// in-app purchase's schedule, though a different Apple resource with its own
// price point catalogue.
//
// A free app is not the absence of a schedule: it is a schedule whose price
// point costs nothing.
type AppPriceSchedule struct {
	Type          string                         `json:"type"`
	ID            string                         `json:"id"`
	Relationships *AppPriceScheduleRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                 `json:"links,omitempty"`
}

type AppPriceScheduleRelationships struct {
	App             *ResourceIdentifier  `json:"app,omitempty"`
	BaseTerritory   *ResourceIdentifier  `json:"baseTerritory,omitempty"`
	ManualPrices    *ResourceIdentifiers `json:"manualPrices,omitempty"`
	AutomaticPrices *ResourceIdentifiers `json:"automaticPrices,omitempty"`
}

// AppPriceScheduleCreateRequest uses JSON:API's "included" member, the same way
// the in-app purchase schedule does.
//
// The prices do not exist yet when the schedule is created, so each travels
// inline under a client-chosen placeholder ID that the manualPrices
// relationship references; Apple substitutes real IDs on commit.
type AppPriceScheduleCreateRequest struct {
	Data     AppPriceScheduleCreateData `json:"data"`
	Included []AppPriceInlineCreate     `json:"included,omitempty"`
}

type AppPriceScheduleCreateData struct {
	Type          string                              `json:"type"`
	Relationships AppPriceScheduleCreateRelationships `json:"relationships"`
}

type AppPriceScheduleCreateRelationships struct {
	App           ResourceIdentifier  `json:"app"`
	BaseTerritory ResourceIdentifier  `json:"baseTerritory"`
	ManualPrices  ResourceIdentifiers `json:"manualPrices"`
}

// AppPriceInlineCreate is one price inside the "included" array.
//
// Unlike the in-app purchase equivalent it names only the price point: an app
// price has no second relationship repeating the parent.
type AppPriceInlineCreate struct {
	Type          string                            `json:"type"`
	ID            string                            `json:"id"`
	Attributes    *AppPriceInlineCreateAttributes   `json:"attributes,omitempty"`
	Relationships AppPriceInlineCreateRelationships `json:"relationships"`
}

// AppPriceInlineCreateAttributes carries plain dates (YYYY-MM-DD), not
// timestamps. A null startDate means the price is in effect now.
type AppPriceInlineCreateAttributes struct {
	StartDate *string `json:"startDate"`
	EndDate   *string `json:"endDate"`
}

type AppPriceInlineCreateRelationships struct {
	AppPricePoint ResourceIdentifier `json:"appPricePoint"`
}

// AppPrice is one scheduled price in a territory.
//
// "manual" distinguishes a price the developer set from one Apple equalized
// from the base territory. Only manual prices are the provider's to manage.
type AppPrice struct {
	Type          string                 `json:"type"`
	ID            string                 `json:"id"`
	Attributes    AppPriceAttributes     `json:"attributes"`
	Relationships *AppPriceRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks         `json:"links,omitempty"`
}

type AppPriceAttributes struct {
	StartDate *string `json:"startDate,omitempty"`
	EndDate   *string `json:"endDate,omitempty"`
	Manual    *bool   `json:"manual,omitempty"`
}

type AppPriceRelationships struct {
	Territory     *ResourceIdentifier `json:"territory,omitempty"`
	AppPricePoint *ResourceIdentifier `json:"appPricePoint,omitempty"`
}

// AppPricePoint is Apple's catalogue of permitted app prices.
//
// A price is never a number: it is a reference to one of these, each fixing a
// customer price and the developer proceeds for one territory. An app's price
// points are read through the app itself and are not interchangeable with a
// subscription's or an in-app purchase's.
type AppPricePoint struct {
	Type          string                      `json:"type"`
	ID            string                      `json:"id"`
	Attributes    AppPricePointAttributes     `json:"attributes"`
	Relationships *AppPricePointRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks              `json:"links,omitempty"`
}

type AppPricePointAttributes struct {
	CustomerPrice string `json:"customerPrice,omitempty"`
	Proceeds      string `json:"proceeds,omitempty"`
}

type AppPricePointRelationships struct {
	App       *ResourceIdentifier `json:"app,omitempty"`
	Territory *ResourceIdentifier `json:"territory,omitempty"`
}

// AppAvailability is the set of territories an app is sold in, Apple's
// AppAvailabilityV2.
//
// It is a different resource from an in-app purchase's or a subscription's
// availability, and a richer one: where those carry a flat list of territory
// codes, this carries a TerritoryAvailability record per storefront, each with
// its own release date and pre-order settings.
type AppAvailability struct {
	Type          string                        `json:"type"`
	ID            string                        `json:"id"`
	Attributes    AppAvailabilityAttributes     `json:"attributes"`
	Relationships *AppAvailabilityRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                `json:"links,omitempty"`
}

type AppAvailabilityAttributes struct {
	AvailableInNewTerritories *bool `json:"availableInNewTerritories,omitempty"`
}

type AppAvailabilityRelationships struct {
	App                     *ResourceIdentifier  `json:"app,omitempty"`
	TerritoryAvailabilities *ResourceIdentifiers `json:"territoryAvailabilities,omitempty"`
}

// TerritoryAvailability is one storefront's entry in an app's availability.
type TerritoryAvailability struct {
	Type          string                              `json:"type"`
	ID            string                              `json:"id"`
	Attributes    TerritoryAvailabilityAttributes     `json:"attributes"`
	Relationships *TerritoryAvailabilityRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks                      `json:"links,omitempty"`
}

type TerritoryAvailabilityAttributes struct {
	Available           *bool    `json:"available,omitempty"`
	ReleaseDate         *string  `json:"releaseDate,omitempty"`
	PreOrderEnabled     *bool    `json:"preOrderEnabled,omitempty"`
	PreOrderPublishDate *string  `json:"preOrderPublishDate,omitempty"`
	ContentStatuses     []string `json:"contentStatuses,omitempty"`
}

type TerritoryAvailabilityRelationships struct {
	Territory *ResourceIdentifier `json:"territory,omitempty"`
}

// AppAvailabilityCreateRequest replaces an app's availability wholesale.
//
// Like the price schedule it sends its children inline: each territory travels
// in "included" under a placeholder ID that the territoryAvailabilities
// relationship references.
type AppAvailabilityCreateRequest struct {
	Data     AppAvailabilityCreateData           `json:"data"`
	Included []TerritoryAvailabilityInlineCreate `json:"included,omitempty"`
}

type AppAvailabilityCreateData struct {
	Type          string                             `json:"type"`
	Attributes    AppAvailabilityCreateAttributes    `json:"attributes"`
	Relationships AppAvailabilityCreateRelationships `json:"relationships"`
}

// AppAvailabilityCreateAttributes takes availableInNewTerritories as a plain
// bool: Apple documents it as required, so it is always sent.
type AppAvailabilityCreateAttributes struct {
	AvailableInNewTerritories bool `json:"availableInNewTerritories"`
}

type AppAvailabilityCreateRelationships struct {
	App                     ResourceIdentifier  `json:"app"`
	TerritoryAvailabilities ResourceIdentifiers `json:"territoryAvailabilities"`
}

// TerritoryAvailabilityInlineCreate is one territory inside "included".
type TerritoryAvailabilityInlineCreate struct {
	Type          string                                       `json:"type"`
	ID            string                                       `json:"id"`
	Attributes    *TerritoryAvailabilityInlineCreateAttributes `json:"attributes,omitempty"`
	Relationships TerritoryAvailabilityInlineRelationships     `json:"relationships"`
}

type TerritoryAvailabilityInlineCreateAttributes struct {
	Available       *bool   `json:"available,omitempty"`
	ReleaseDate     *string `json:"releaseDate,omitempty"`
	PreOrderEnabled *bool   `json:"preOrderEnabled,omitempty"`
}

type TerritoryAvailabilityInlineRelationships struct {
	Territory ResourceIdentifier `json:"territory"`
}
