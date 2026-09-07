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
type MerchantIDAttributes struct {
	Identifier  string `json:"identifier"`
	DisplayName string `json:"displayName"`
}

// Update attributes (only displayName can be updated for Merchant IDs).
type MerchantIDUpdateAttributes struct {
	DisplayName string `json:"displayName"`
}
