package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"terraform-provider-apple/internal/apple/models"
)

// GetMerchantIDs retrieves all Merchant IDs for the team
func (c *Client) GetMerchantIDs() ([]models.MerchantID, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/merchantIds", c.HostURL), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.ListResponse[models.MerchantID]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetMerchantID retrieves a specific Merchant ID by its ID
func (c *Client) GetMerchantID(merchantID string) (*models.MerchantID, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/merchantIds/%s", c.HostURL, merchantID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.MerchantID]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateMerchantID creates a new Merchant ID
func (c *Client) CreateMerchantID(identifier, displayName string, authToken *string) (*models.MerchantID, error) {
	merchantIDRequest := models.MerchantIDCreateRequest{
		Type: "merchantIds",
		Attributes: models.MerchantIDAttributes{
			Identifier:  identifier,
			DisplayName: displayName,
		},
	}

	requestData := models.Request[models.MerchantIDCreateRequest]{
		Data: merchantIDRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/merchantIds", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.MerchantID]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateMerchantID updates a Merchant ID's display name (only field that can be updated)
func (c *Client) UpdateMerchantID(merchantID, displayName string, authToken *string) (*models.MerchantID, error) {
	merchantIDRequest := models.MerchantIDUpdateRequest{
		Type: "merchantIds",
		ID:   merchantID,
		Attributes: models.MerchantIDUpdateAttributes{
			DisplayName: displayName,
		},
	}

	requestData := models.Request[models.MerchantIDUpdateRequest]{
		Data: merchantIDRequest,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/merchantIds/%s", c.HostURL, merchantID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.MerchantID]{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteMerchantID deletes a Merchant ID
func (c *Client) DeleteMerchantID(merchantID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/merchantIds/%s", c.HostURL, merchantID), nil)
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

// GetMerchantIDByIdentifier finds a Merchant ID by its identifier string
func (c *Client) GetMerchantIDByIdentifier(identifier string) (*models.MerchantID, error) {
	merchantIDs, err := c.GetMerchantIDs()
	if err != nil {
		return nil, err
	}

	for _, merchantID := range merchantIDs {
		if merchantID.Attributes.Identifier == identifier {
			return &merchantID, nil
		}
	}

	return nil, fmt.Errorf("merchant ID with identifier '%s' not found", identifier)
}
