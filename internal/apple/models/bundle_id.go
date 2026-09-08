// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Bundle ID platform enumeration.
type BundleIDPlatform string

// Apple accepts only these three on POST /v1/bundleIds; TV_OS and WATCH_OS are
// rejected with "'TV_OS' is not a valid value for the attribute 'platform'.
// Expected one of: 'IOS', 'MAC_OS', 'UNIVERSAL'". tvOS and watchOS App IDs are
// created as UNIVERSAL, which is also what Apple stores for every value it does
// accept.
const (
	IOS       BundleIDPlatform = "IOS"
	MACOS     BundleIDPlatform = "MAC_OS"
	UNIVERSAL BundleIDPlatform = "UNIVERSAL"
)

// Bundle ID resource model.
type BundleID struct {
	Type       string             `json:"type"`
	ID         string             `json:"id"`
	Attributes BundleIDAttributes `json:"attributes"`
	Links      *ResourceLinks     `json:"links,omitempty"`
}

// Request models for Bundle ID operations.
type BundleIDCreateRequest struct {
	Type       string             `json:"type"`
	Attributes BundleIDAttributes `json:"attributes"`
}

type BundleIDUpdateRequest struct {
	Type       string                   `json:"type"`
	ID         string                   `json:"id"`
	Attributes BundleIDUpdateAttributes `json:"attributes"`
}

// Bundle ID attributes.
type BundleIDAttributes struct {
	Identifier string           `json:"identifier"`
	Name       string           `json:"name"`
	Platform   BundleIDPlatform `json:"platform"`
	SeedID     string           `json:"seedId,omitempty"`
}

// Update attributes (only name can be updated for Bundle IDs).
type BundleIDUpdateAttributes struct {
	Name string `json:"name"`
}
