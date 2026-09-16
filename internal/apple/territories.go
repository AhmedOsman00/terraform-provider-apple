// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// GetTerritories retrieves every territory the App Store operates in.
//
// Unlike the other App Store Connect collections this provider reads,
// /v1/territories is a genuine top-level collection rather than a relationship
// hanging off an app -- there is no parent to scope it to, and no credentials
// dependence beyond the token: every account sees the same list.
//
// The list is not the same thing as Apple's TerritoryCode type. That type
// enumerates every code the API will parse, including storefronts the App Store
// does not operate in; this returns the ones a product can actually be sold in,
// which is what an availability record has to be built from.
func (c *Client) GetTerritories() ([]models.Territory, error) {
	return getAllPages[models.Territory](c, "/v1/territories", defaultPageSize)
}
