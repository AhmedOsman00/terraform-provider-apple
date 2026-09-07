// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Generic API response wrappers for App Store Connect API.
type ListResponse[T any] struct {
	Data  []T                 `json:"data"`
	Links *PagedDocumentLinks `json:"links,omitempty"`
	Meta  *PagingInformation  `json:"meta,omitempty"`
}

type Response[T any] struct {
	Data T `json:"data"`
}

type Request[T any] struct {
	Data T `json:"data"`
}

// Error handling models.
type ErrorResponse struct {
	Errors []APIError `json:"errors"`
}

type APIError struct {
	ID     string                 `json:"id,omitempty"`
	Status string                 `json:"status"`
	Code   string                 `json:"code"`
	Title  string                 `json:"title"`
	Detail string                 `json:"detail"`
	Source map[string]interface{} `json:"source,omitempty"`
}

// Pagination models.
type PagedDocumentLinks struct {
	Self  string `json:"self,omitempty"`
	First string `json:"first,omitempty"`
	Next  string `json:"next,omitempty"`
}

type PagingInformation struct {
	Total int `json:"total,omitempty"`
}

// Common resource models.
type ResourceLinks struct {
	Self string `json:"self,omitempty"`
}
