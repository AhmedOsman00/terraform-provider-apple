// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

const (
	// defaultPageSize is the largest page App Store Connect returns for the
	// top-level collections this provider reads.
	defaultPageSize = 200

	// noPageSize omits the limit parameter entirely. Some relationship
	// collections reject it outright -- bundleIdCapabilities answers 400 "The
	// parameter 'limit' can not be used with this request : This relationship
	// does not support this parameter" -- and return the whole set in one
	// response instead. Pagination still follows any "next" link that appears.
	noPageSize = 0

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
	return getAllPagesQuery[T](c, endpoint, pageSize, nil)
}

// getAllPagesQuery is getAllPages with additional query parameters on the first
// request. The "next" link Apple returns already carries them forward, so they
// are applied once rather than re-appended per page.
//
// The subscription collections need this: a price is unreadable without
// include=subscriptionPricePoint, and the price point catalogue is large enough
// that filter[territory] is the difference between one page and hundreds.
func getAllPagesQuery[T any](c *Client, endpoint string, pageSize int, params url.Values) ([]T, error) {
	query := url.Values{}
	for k, v := range params {
		query[k] = v
	}
	if pageSize > 0 {
		query.Set("limit", strconv.Itoa(pageSize))
	}

	next := c.HostURL + endpoint
	if encoded := query.Encode(); encoded != "" {
		next += "?" + encoded
	}

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
