// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

const (
	// POST /v1/profiles intermittently answers 500 "An unexpected error
	// occurred" for a request that is well formed and succeeds on the next
	// attempt. It is not tied to the profile name, the bundle, or the
	// certificate -- issuing the same request repeatedly fails at random.
	// Terraform surfaces that as a failed apply, so the create is retried.
	//
	// Only profile creation retries. Everything else this client does either
	// succeeds or fails deterministically, and a blanket retry would hide real
	// errors and repeat non-idempotent writes.
	profileCreateAttempts = 3
	profileCreateBackoff  = 2 * time.Second
)

// isRetryableServerError reports whether an error came back as a 5xx.
//
// doRequest renders Apple's errors as text, so this matches on the status it
// formats in. A 4xx is the caller's fault and never retried.
func isRetryableServerError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	for _, status := range []string{"status 500", "status 502", "status 503", "status 504"} {
		if strings.Contains(msg, status) {
			return true
		}
	}

	return false
}

// GetProfiles retrieves all Profiles for the team.
func (c *Client) GetProfiles() ([]models.Profile, error) {
	return getAllPages[models.Profile](c, "/v1/profiles", defaultPageSize)
}

// GetProfile retrieves a specific Profile by its ID.
func (c *Client) GetProfile(profileID string) (*models.Profile, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/profiles/%s", c.HostURL, profileID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Profile]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateProfile creates a new Profile.
func (c *Client) CreateProfile(name string, profileType models.ProfileType, bundleIDID string, certificateIDs []string, deviceIDs []string, authToken *string) (*models.Profile, error) {
	// Convert certificate IDs to resource identifiers
	certificates := models.ResourceIdentifiers{
		Data: make([]models.ResourceData, len(certificateIDs)),
	}
	for i, certID := range certificateIDs {
		certificates.Data[i] = models.ResourceData{
			Type: "certificates",
			ID:   certID,
		}
	}

	// Convert device IDs to resource identifiers. App Store and Developer ID
	// profiles have no devices, and Apple rejects an empty devices relationship,
	// so the key is left out entirely rather than sent as an empty list.
	var devices *models.ResourceIdentifiers
	if len(deviceIDs) > 0 {
		devices = &models.ResourceIdentifiers{
			Data: make([]models.ResourceData, len(deviceIDs)),
		}
		for i, deviceID := range deviceIDs {
			devices.Data[i] = models.ResourceData{
				Type: "devices",
				ID:   deviceID,
			}
		}
	}

	profileRequest := models.ProfileCreateRequest{
		Type: "profiles",
		Attributes: models.ProfileCreateAttributes{
			Name:        name,
			ProfileType: profileType,
		},
		Relationships: models.ProfileCreateRelationships{
			BundleID: models.ResourceIdentifier{
				Data: models.ResourceData{
					Type: "bundleIds",
					ID:   bundleIDID,
				},
			},
			Certificates: certificates,
			Devices:      devices,
		},
	}

	requestData := models.Request[models.ProfileCreateRequest]{
		Data: profileRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < profileCreateAttempts; attempt++ {
		if attempt > 0 {
			// A 500 sometimes lands after Apple has already issued the profile.
			// Look before retrying: the name is unique per team, so finding it
			// means the previous attempt succeeded and a retry would either
			// duplicate it or fail on the name.
			if existing, err := c.GetProfileByName(name); err == nil {
				return existing, nil
			}

			time.Sleep(profileCreateBackoff)
		}

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/profiles", c.HostURL), strings.NewReader(string(rb)))
		if err != nil {
			return nil, err
		}

		body, err := c.doRequest(req, authToken)
		if err != nil {
			lastErr = err
			if isRetryableServerError(err) {
				continue
			}

			return nil, err
		}

		response := models.Response[models.Profile]{}
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, err
		}

		return &response.Data, nil
	}

	return nil, fmt.Errorf("creating profile %q failed after %d attempts: %w", name, profileCreateAttempts, lastErr)
}

// UpdateProfile updates a Profile's name (only field that can be updated).
func (c *Client) UpdateProfile(profileID, name string, authToken *string) (*models.Profile, error) {
	profileRequest := models.ProfileUpdateRequest{
		Type: "profiles",
		ID:   profileID,
		Attributes: models.ProfileUpdateAttributes{
			Name: name,
		},
	}

	requestData := models.Request[models.ProfileUpdateRequest]{
		Data: profileRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/profiles/%s", c.HostURL, profileID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Profile]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteProfile deletes a Profile.
func (c *Client) DeleteProfile(profileID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/profiles/%s", c.HostURL, profileID), nil)
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

// GetProfileByName finds a Profile by its name.
func (c *Client) GetProfileByName(name string) (*models.Profile, error) {
	profiles, err := c.GetProfiles()
	if err != nil {
		return nil, err
	}

	for _, profile := range profiles {
		if profile.Attributes.Name == name {
			return &profile, nil
		}
	}

	return nil, fmt.Errorf("profile with name '%s' not found", name)
}
