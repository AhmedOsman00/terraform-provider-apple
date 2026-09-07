// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// GetPassTypeIDs retrieves all Pass Type IDs for the team.
func (c *Client) GetPassTypeIDs() ([]models.PassTypeIDResource, error) {
	return getAllPages[models.PassTypeIDResource](c, "/v1/passTypeIds", defaultPageSize)
}

// GetPassTypeID retrieves a specific Pass Type ID by its ID.
func (c *Client) GetPassTypeID(passTypeID string) (*models.PassTypeIDResource, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/passTypeIds/%s", c.HostURL, passTypeID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.PassTypeIDResource]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetPassTypeIDByIdentifier retrieves a specific Pass Type ID by its identifier (e.g., "pass.com.example.mypass").
func (c *Client) GetPassTypeIDByIdentifier(identifier string) (*models.PassTypeIDResource, error) {
	passTypeIDs, err := c.GetPassTypeIDs()
	if err != nil {
		return nil, err
	}

	for _, passTypeID := range passTypeIDs {
		if passTypeID.Attributes.Identifier == identifier {
			return &passTypeID, nil
		}
	}

	return nil, fmt.Errorf("pass type ID with identifier %q not found", identifier)
}

// CreatePassTypeID creates a new Pass Type ID.
func (c *Client) CreatePassTypeID(identifier, name string) (*models.PassTypeIDResource, error) {
	requestBody := models.Request[models.PassTypeIDCreateRequest]{
		Data: models.PassTypeIDCreateRequest{
			Type: "passTypeIds",
			Attributes: models.PassTypeIDAttributes{
				Identifier: identifier,
				Name:       name,
			},
		},
	}

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/passTypeIds", c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.PassTypeIDResource]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdatePassTypeID updates an existing Pass Type ID (only name can be updated).
func (c *Client) UpdatePassTypeID(passTypeID, name string) (*models.PassTypeIDResource, error) {
	requestBody := models.Request[models.PassTypeIDUpdateRequest]{
		Data: models.PassTypeIDUpdateRequest{
			Type: "passTypeIds",
			ID:   passTypeID,
			Attributes: models.PassTypeIDUpdateAttributes{
				Name: name,
			},
		},
	}

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/passTypeIds/%s", c.HostURL, passTypeID), bytes.NewReader(rb))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.PassTypeIDResource]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeletePassTypeID deletes a Pass Type ID.
func (c *Client) DeletePassTypeID(passTypeID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/passTypeIds/%s", c.HostURL, passTypeID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, nil)
	if err != nil {
		// Handle specific error cases
		if strings.Contains(err.Error(), "404") {
			return fmt.Errorf("pass type ID not found (it may have already been deleted)")
		}
		return err
	}

	return nil
}
