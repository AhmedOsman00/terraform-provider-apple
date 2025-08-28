package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-provider-apple/internal/apple/models"
)

// GetProfiles retrieves all Profiles for the team
func (c *Client) GetProfiles() ([]models.Profile, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/profiles", c.HostURL), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.ListResponse[models.Profile]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetProfile retrieves a specific Profile by its ID
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

// CreateProfile creates a new Profile
func (c *Client) CreateProfile(name string, platform models.ProfilePlatform, bundleIDID string, certificateIDs []string, deviceIDs []string, authToken *string) (*models.Profile, error) {
	// Convert certificate IDs to resource identifiers
	certificates := make([]models.ResourceIdentifier, len(certificateIDs))
	for i, certID := range certificateIDs {
		certificates[i] = models.ResourceIdentifier{
			Data: models.ResourceData{
				Type: "certificates",
				ID:   certID,
			},
		}
	}

	// Convert device IDs to resource identifiers (optional)
	devices := make([]models.ResourceIdentifier, len(deviceIDs))
	for i, deviceID := range deviceIDs {
		devices[i] = models.ResourceIdentifier{
			Data: models.ResourceData{
				Type: "devices",
				ID:   deviceID,
			},
		}
	}

	profileRequest := models.ProfileCreateRequest{
		Type: "profiles",
		Attributes: models.ProfileCreateAttributes{
			Name:     name,
			Platform: platform,
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

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/profiles", c.HostURL), strings.NewReader(string(rb)))
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

// UpdateProfile updates a Profile's name (only field that can be updated)
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

// DeleteProfile deletes a Profile
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

// GetProfileByName finds a Profile by its name
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
