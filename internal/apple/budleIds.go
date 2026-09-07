// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// GetBundleIDs retrieves all Bundle IDs for the team.
func (c *Client) GetBundleIDs() ([]models.BundleID, error) {
	return getAllPages[models.BundleID](c, "/v1/bundleIds", defaultPageSize)
}

// GetBundleID retrieves a specific Bundle ID by its ID.
func (c *Client) GetBundleID(bundleID string) (*models.BundleID, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/bundleIds/%s", c.HostURL, bundleID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BundleID]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateBundleID creates a new Bundle ID.
func (c *Client) CreateBundleID(identifier, name string, platform models.BundleIDPlatform, authToken *string) (*models.BundleID, error) {
	bundleIDRequest := models.BundleIDCreateRequest{
		Type: "bundleIds",
		Attributes: models.BundleIDAttributes{
			Identifier: identifier,
			Name:       name,
			Platform:   platform,
		},
	}

	requestData := models.Request[models.BundleIDCreateRequest]{
		Data: bundleIDRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/bundleIds", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BundleID]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateBundleID updates a Bundle ID's name (only field that can be updated).
func (c *Client) UpdateBundleID(bundleID, name string, authToken *string) (*models.BundleID, error) {
	bundleIDRequest := models.BundleIDUpdateRequest{
		Type: "bundleIds",
		ID:   bundleID,
		Attributes: models.BundleIDUpdateAttributes{
			Name: name,
		},
	}

	requestData := models.Request[models.BundleIDUpdateRequest]{
		Data: bundleIDRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/bundleIds/%s", c.HostURL, bundleID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BundleID]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteBundleID deletes a Bundle ID.
func (c *Client) DeleteBundleID(bundleID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/bundleIds/%s", c.HostURL, bundleID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	if err != nil {
		return err
	}

	// DELETE requests return 204 No Content on success, which is handled by doRequest
	return nil
}

// GetBundleIDByIdentifier finds a Bundle ID by its identifier string.
func (c *Client) GetBundleIDByIdentifier(identifier string) (*models.BundleID, error) {
	bundleIDs, err := c.GetBundleIDs()
	if err != nil {
		return nil, err
	}

	for _, bundleID := range bundleIDs {
		if bundleID.Attributes.Identifier == identifier {
			return &bundleID, nil
		}
	}

	return nil, fmt.Errorf("bundle ID with identifier '%s' not found", identifier)
}
