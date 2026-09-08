// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Merchant ID resource model.
type MerchantID struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes MerchantIDAttributes `json:"attributes"`
	Links      *ResourceLinks       `json:"links,omitempty"`
}

// Request models for Merchant ID operations.
type MerchantIDCreateRequest struct {
	Type       string               `json:"type"`
	Attributes MerchantIDAttributes `json:"attributes"`
}

type MerchantIDUpdateRequest struct {
	Type       string                     `json:"type"`
	ID         string                     `json:"id"`
	Attributes MerchantIDUpdateAttributes `json:"attributes"`
}

// Merchant ID attributes.
//
// Apple calls the human-readable field "name" on merchantIds; sending
// "displayName" is rejected with a 409 ENTITY_ERROR.ATTRIBUTE.UNKNOWN and a
// pointer at /data/attributes/displayName. The Go field and the Terraform
// attribute stay display_name because that is the provider's published schema
// and Apple's own portal labels it that way -- only the wire tag is "name".
type MerchantIDAttributes struct {
	Identifier  string `json:"identifier"`
	DisplayName string `json:"name"`
	Prefix      string `json:"prefix,omitempty"`
}

// Update attributes (only the display name can be updated for Merchant IDs).
type MerchantIDUpdateAttributes struct {
	DisplayName string `json:"name"`
}
