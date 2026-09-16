// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Territory resource model.
//
// A territory is an App Store storefront, identified by a three-letter
// uppercase code -- USA, GBR, EGY -- not the two-letter ISO 3166-1 alpha-2
// code. The ID is the code itself, which is why a territory relationship can be
// written without looking anything up first.
//
// Territories are read-only -- they belong to the App Store rather than to the
// account, so there is no resource and cannot be one. The provider reads them
// in two places: the apple_territories data source, which lists the storefronts
// a product may be sold in, and the territory lists an availability record
// covers.
type Territory struct {
	Type       string              `json:"type"`
	ID         string              `json:"id"`
	Attributes TerritoryAttributes `json:"attributes"`
	Links      *ResourceLinks      `json:"links,omitempty"`
}

// TerritoryAttributes holds the territory's only attribute.
type TerritoryAttributes struct {
	Currency string `json:"currency,omitempty"`
}
