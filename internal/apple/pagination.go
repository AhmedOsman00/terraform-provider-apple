// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"terraform-provider-apple/internal/apple/models"
)

const (
	// defaultPageSize is the largest page App Store Connect returns for the
	// top-level collections this provider reads.
	defaultPageSize = 200

	// relationshipPageSize is used for sub-resource collections, which cap the
	// limit parameter lower than the top-level collections do. Requesting more
	// than an endpoint allows is rejected with a parameter error, so this stays
	// conservative.
	relationshipPageSize = 50

	// maxPages bounds the walk so a malformed or self-referential "next" link
	// cannot spin forever.
	maxPages = 1000
)

// getAllPages walks every page of a paginated App Store Connect collection and
// returns the accumulated records.
//
// Apple returns at most pageSize records per response and supplies an absolute
// "next" link for as long as further pages remain. Requesting only the first
// page silently truncates results, which matters both for the list data sources
// and for the lookup helpers that scan a full collection to resolve a
// human-readable identifier.
func getAllPages[T any](c *Client, endpoint string, pageSize int) ([]T, error) {
	next := fmt.Sprintf("%s%s?limit=%d", c.HostURL, endpoint, pageSize)

	var all []T
	for page := 0; next != ""; page++ {
		if page >= maxPages {
			return nil, fmt.Errorf("pagination exceeded %d pages for %s, aborting", maxPages, endpoint)
		}

		req, err := http.NewRequest("GET", next, nil)
		if err != nil {
			return nil, err
		}

		body, err := c.doRequest(req, nil)
		if err != nil {
			return nil, err
		}

		response := models.ListResponse[T]{}
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, err
		}

		all = append(all, response.Data...)

		next = ""
		if response.Links != nil && response.Links.Next != "" {
			if err := c.checkSameHost(response.Links.Next); err != nil {
				return nil, err
			}
			next = response.Links.Next
		}
	}

	return all, nil
}

// checkSameHost verifies a pagination link points back at the host this client
// was configured for. The link is taken from a response body, and the bearer
// token must never be replayed to a host the client was not pointed at.
func (c *Client) checkSameHost(rawURL string) error {
	next, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid pagination link %q: %v", rawURL, err)
	}

	base, err := url.Parse(c.HostURL)
	if err != nil {
		return fmt.Errorf("invalid client host URL %q: %v", c.HostURL, err)
	}

	if next.Scheme != base.Scheme || next.Host != base.Host {
		return fmt.Errorf("pagination link %q points outside %s, refusing to follow", rawURL, c.HostURL)
	}

	return nil
}
