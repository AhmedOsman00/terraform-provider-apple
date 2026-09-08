// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// App resource model.
//
// Apps are read-only here on purpose: Apple's documentation for POST endpoints
// states "Don't use this API to create new apps; instead, create new apps on
// the App Store Connect website." The provider exposes apps only so that a
// subscription group can resolve the app it belongs to.
type App struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Attributes AppAttributes  `json:"attributes"`
	Links      *ResourceLinks `json:"links,omitempty"`
}

// App attributes.
//
// Only the identifying fields are modelled. An app carries a great deal more
// metadata (age ratings, encryption declarations, pricing), none of which this
// provider manages.
type AppAttributes struct {
	Name          string `json:"name,omitempty"`
	BundleID      string `json:"bundleId,omitempty"`
	SKU           string `json:"sku,omitempty"`
	PrimaryLocale string `json:"primaryLocale,omitempty"`
}
