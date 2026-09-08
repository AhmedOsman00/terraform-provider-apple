// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// GetApps retrieves every app visible to the credentials.
func (c *Client) GetApps() ([]models.App, error) {
	return getAllPages[models.App](c, "/v1/apps", defaultPageSize)
}

// GetApp retrieves a specific app by its Apple ID.
func (c *Client) GetApp(appID string) (*models.App, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/apps/%s", c.HostURL, appID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.App]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetAppByBundleID finds an app by its bundle identifier.
//
// Unlike the Developer Portal collections this provider reads elsewhere,
// /v1/apps does support server-side filtering, so this asks Apple to do the
// matching rather than listing every app and scanning. filter[bundleId] is an
// exact match.
func (c *Client) GetAppByBundleID(bundleID string) (*models.App, error) {
	params := url.Values{}
	params.Set("filter[bundleId]", bundleID)

	apps, err := getAllPagesQuery[models.App](c, "/v1/apps", defaultPageSize, params)
	if err != nil {
		return nil, err
	}

	// filter[bundleId] is documented as exact, but confirm it rather than
	// trusting the first record: a silently ignored filter would otherwise
	// return an arbitrary app.
	for _, app := range apps {
		if app.Attributes.BundleID == bundleID {
			return &app, nil
		}
	}

	return nil, fmt.Errorf("app with bundle ID '%s' not found. The app record must be created in App Store Connect first -- Apple's API cannot create one", bundleID)
}
