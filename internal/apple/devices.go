// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-provider-apple/internal/apple/models"
)

// GetDevices retrieves all Devices for the team.
func (c *Client) GetDevices() ([]models.Device, error) {
	return getAllPages[models.Device](c, "/v1/devices", defaultPageSize)
}

// GetDevice retrieves a specific Device by its ID.
func (c *Client) GetDevice(deviceID string) (*models.Device, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/devices/%s", c.HostURL, deviceID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Device]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateDevice creates a new Device.
func (c *Client) CreateDevice(name, udid string, platform models.DevicePlatform, authToken *string) (*models.Device, error) {
	deviceRequest := models.DeviceCreateRequest{
		Type: "devices",
		Attributes: models.DeviceCreateAttributes{
			Name:     name,
			UDID:     udid,
			Platform: platform,
		},
	}

	requestData := models.Request[models.DeviceCreateRequest]{
		Data: deviceRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/devices", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Device]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateDevice updates a Device's name and optionally status.
func (c *Client) UpdateDevice(deviceID, name string, status *models.DeviceStatus, authToken *string) (*models.Device, error) {
	deviceRequest := models.DeviceUpdateRequest{
		Type: "devices",
		ID:   deviceID,
		Attributes: models.DeviceUpdateAttributes{
			Name:   name,
			Status: status,
		},
	}

	requestData := models.Request[models.DeviceUpdateRequest]{
		Data: deviceRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/devices/%s", c.HostURL, deviceID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Device]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetDeviceByUDID finds a Device by its UDID string.
func (c *Client) GetDeviceByUDID(udid string) (*models.Device, error) {
	devices, err := c.GetDevices()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		if device.Attributes.UDID == udid {
			return &device, nil
		}
	}

	return nil, fmt.Errorf("device with UDID '%s' not found", udid)
}
