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

// GetBundleIDCapabilities retrieves all capabilities for a specific Bundle ID.
func (c *Client) GetBundleIDCapabilities(bundleID string) ([]models.BundleIDCapability, error) {
	return getAllPages[models.BundleIDCapability](c, fmt.Sprintf("/v1/bundleIds/%s/bundleIdCapabilities", bundleID), relationshipPageSize)
}

// GetBundleIDCapability retrieves a specific Bundle ID capability by its ID.
func (c *Client) GetBundleIDCapability(capabilityID string) (*models.BundleIDCapability, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/bundleIdCapabilities/%s", c.HostURL, capabilityID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BundleIDCapability]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateBundleIDCapability creates a new Bundle ID capability.
func (c *Client) CreateBundleIDCapability(bundleID string, capabilityType models.CapabilityType, settings []models.CapabilitySetting, authToken *string) (*models.BundleIDCapability, error) {
	capabilityRequest := models.BundleIDCapabilityCreateRequest{
		Type: "bundleIdCapabilities",
		Attributes: models.BundleIDCapabilityAttributes{
			CapabilityType: capabilityType,
			Settings:       settings,
		},
	}

	requestData := struct {
		Data          models.BundleIDCapabilityCreateRequest `json:"data"`
		Relationships struct {
			BundleID struct {
				Data struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"data"`
			} `json:"bundleId"`
		} `json:"relationships"`
	}{
		Data: capabilityRequest,
	}

	// Set the bundle ID relationship
	requestData.Relationships.BundleID.Data.Type = "bundleIds"
	requestData.Relationships.BundleID.Data.ID = bundleID

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/bundleIdCapabilities", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BundleIDCapability]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateBundleIDCapability updates a Bundle ID capability's settings.
func (c *Client) UpdateBundleIDCapability(capabilityID string, capabilityType *models.CapabilityType, settings []models.CapabilitySetting, authToken *string) (*models.BundleIDCapability, error) {
	capabilityRequest := models.BundleIDCapabilityUpdateRequest{
		Type: "bundleIdCapabilities",
		ID:   capabilityID,
		Attributes: models.BundleIDCapabilityUpdateAttributes{
			CapabilityType: capabilityType,
			Settings:       settings,
		},
	}

	requestData := models.Request[models.BundleIDCapabilityUpdateRequest]{
		Data: capabilityRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/bundleIdCapabilities/%s", c.HostURL, capabilityID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BundleIDCapability]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteBundleIDCapability deletes a Bundle ID capability.
func (c *Client) DeleteBundleIDCapability(capabilityID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/bundleIdCapabilities/%s", c.HostURL, capabilityID), nil)
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

// GetBundleIDCapabilityByType finds a Bundle ID capability by bundle ID and capability type.
func (c *Client) GetBundleIDCapabilityByType(bundleID string, capabilityType models.CapabilityType) (*models.BundleIDCapability, error) {
	capabilities, err := c.GetBundleIDCapabilities(bundleID)
	if err != nil {
		return nil, err
	}

	for _, capability := range capabilities {
		if capability.Attributes.CapabilityType == capabilityType {
			return &capability, nil
		}
	}

	return nil, fmt.Errorf("bundle ID capability with type '%s' not found for bundle ID '%s'", capabilityType, bundleID)
}
