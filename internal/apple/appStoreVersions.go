// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// GetAppStoreVersions retrieves every version of an app, optionally narrowed to
// one platform.
//
// Unlike most collections this provider reads, Apple does filter this one
// server-side, and an app accumulates a version record per release for its
// whole life -- so the platform filter is passed through rather than applied in
// memory.
func (c *Client) GetAppStoreVersions(appID, platform string) ([]models.AppStoreVersion, error) {
	params := url.Values{}
	params.Set("include", "app")
	if platform != "" {
		params.Set("filter[platform]", platform)
	}

	return getAllPagesQuery[models.AppStoreVersion](
		c,
		fmt.Sprintf("/v1/apps/%s/appStoreVersions", appID),
		defaultPageSize,
		params,
	)
}

// GetAppStoreVersion retrieves a single version by its ID.
//
// include=app populates the owning app, without which an imported version would
// have a null app_id.
func (c *Client) GetAppStoreVersion(versionID string) (*models.AppStoreVersion, error) {
	params := url.Values{}
	params.Set("include", "app")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/appStoreVersions/%s?%s", c.HostURL, versionID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreVersion]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetAppStoreVersionByString finds a version of an app by its version string
// and platform.
//
// Apple does publish filter[versionString], but it is applied here after the
// platform filter because the same version string can exist once per platform:
// "1.0.0" on IOS and "1.0.0" on MAC_OS are two different records.
func (c *Client) GetAppStoreVersionByString(appID, platform, versionString string) (*models.AppStoreVersion, error) {
	versions, err := c.GetAppStoreVersions(appID, platform)
	if err != nil {
		return nil, err
	}

	for i := range versions {
		if versions[i].Attributes.VersionString == versionString {
			return &versions[i], nil
		}
	}

	return nil, fmt.Errorf("app store version '%s' not found for app '%s' on platform '%s'", versionString, appID, platform)
}

// CreateAppStoreVersion creates a new version of an app.
//
// An app can hold only one editable version per platform at a time: Apple
// answers a second one with a 409 naming the version already being prepared.
func (c *Client) CreateAppStoreVersion(appID string, attributes models.AppStoreVersionCreateAttributes, authToken *string) (*models.AppStoreVersion, error) {
	requestData := models.Request[models.AppStoreVersionCreateRequest]{
		Data: models.AppStoreVersionCreateRequest{
			Type:       "appStoreVersions",
			Attributes: attributes,
			Relationships: models.AppStoreVersionCreateRelationships{
				App: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "apps", ID: appID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/appStoreVersions", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreVersion]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateAppStoreVersion updates a version in place.
//
// platform is absent from Apple's update request: a version belongs to one
// platform for its whole life.
func (c *Client) UpdateAppStoreVersion(versionID string, attributes models.AppStoreVersionUpdateAttributes, authToken *string) (*models.AppStoreVersion, error) {
	requestData := models.Request[models.AppStoreVersionUpdateRequest]{
		Data: models.AppStoreVersionUpdateRequest{
			Type:       "appStoreVersions",
			ID:         versionID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/appStoreVersions/%s", c.HostURL, versionID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreVersion]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteAppStoreVersion deletes a version.
//
// Only an editable version can be deleted. A version that has been released, or
// is in review, is part of the app's history and Apple refuses to remove it.
func (c *Client) DeleteAppStoreVersion(versionID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/appStoreVersions/%s", c.HostURL, versionID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
