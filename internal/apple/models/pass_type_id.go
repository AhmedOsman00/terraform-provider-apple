// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Pass Type ID resource model.
type PassTypeIDResource struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes PassTypeIDAttributes `json:"attributes"`
	Links      *ResourceLinks       `json:"links,omitempty"`
}

// Request models for Pass Type ID operations.
type PassTypeIDCreateRequest struct {
	Type       string               `json:"type"`
	Attributes PassTypeIDAttributes `json:"attributes"`
}

type PassTypeIDUpdateRequest struct {
	Type       string                     `json:"type"`
	ID         string                     `json:"id"`
	Attributes PassTypeIDUpdateAttributes `json:"attributes"`
}

// Pass Type ID attributes.
type PassTypeIDAttributes struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
}

// Update attributes (only name can be updated for Pass Type IDs).
type PassTypeIDUpdateAttributes struct {
	Name string `json:"name"`
}
