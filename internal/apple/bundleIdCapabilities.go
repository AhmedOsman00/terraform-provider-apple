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
	return getAllPages[models.BundleIDCapability](c, fmt.Sprintf("/v1/bundleIds/%s/bundleIdCapabilities", bundleID), noPageSize)
}

// GetBundleIDCapability retrieves a single capability of a Bundle ID.
//
// Apple exposes no GET for an individual capability: requesting one answers
// 403 "The resource 'bundleIdCapabilities' does not allow 'GET_INSTANCE'.
// Allowed operations are: CREATE, DELETE, UPDATE". The parent Bundle ID's
// capability collection is the only readable form, so the lookup lists it and
// scans -- which is also why every caller has to know the parent Bundle ID.
func (c *Client) GetBundleIDCapability(bundleID, capabilityID string) (*models.BundleIDCapability, error) {
	capabilities, err := c.GetBundleIDCapabilities(bundleID)
	if err != nil {
		return nil, err
	}

	for i := range capabilities {
		if capabilities[i].ID == capabilityID {
			return &capabilities[i], nil
		}
	}

	return nil, fmt.Errorf("bundle ID capability %q not found on bundle ID %q", capabilityID, bundleID)
}

// BundleIDFromCapabilityID recovers the parent Bundle ID from a capability ID.
//
// Apple composes capability IDs as "<bundleID>_<CAPABILITY_TYPE>", for example
// "VT4WL83433_PUSH_NOTIFICATIONS". The capability type itself contains
// underscores, so the split is on the first one only. This format is not
// documented, so callers must confirm the result against Apple rather than
// trusting it: an empty string means the ID did not have the expected shape.
func BundleIDFromCapabilityID(capabilityID string) string {
	bundleID, _, found := strings.Cut(capabilityID, "_")
	if !found {
		return ""
	}

	return bundleID
}

// CreateBundleIDCapability creates a new Bundle ID capability.
func (c *Client) CreateBundleIDCapability(bundleID string, capabilityType models.CapabilityType, settings []models.CapabilitySetting, authToken *string) (*models.BundleIDCapability, error) {
	capabilityRequest := models.BundleIDCapabilityCreateRequest{
		Type: "bundleIdCapabilities",
		Attributes: models.BundleIDCapabilityAttributes{
			CapabilityType: capabilityType,
			Settings:       settings,
		},
		Relationships: models.BundleIDCapabilityCreateRelationships{
			BundleID: models.ResourceIdentifier{
				Data: models.ResourceData{
					Type: "bundleIds",
					ID:   bundleID,
				},
			},
		},
	}

	requestData := models.Request[models.BundleIDCapabilityCreateRequest]{
		Data: capabilityRequest,
	}

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
