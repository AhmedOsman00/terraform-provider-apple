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

// GetSubscriptionGroupLocalizations retrieves every localization of a group.
func (c *Client) GetSubscriptionGroupLocalizations(groupID string) ([]models.SubscriptionGroupLocalization, error) {
	return getAllPages[models.SubscriptionGroupLocalization](
		c,
		fmt.Sprintf("/v1/subscriptionGroups/%s/subscriptionGroupLocalizations", groupID),
		defaultPageSize,
	)
}

// GetSubscriptionGroupLocalization retrieves a specific localization by its ID.
//
// include=subscriptionGroup populates the parent linkage. Unlike the group
// itself -- whose owning app can never be read back -- a group localization
// does report its parent, so import accepts a bare ID.
func (c *Client) GetSubscriptionGroupLocalization(localizationID string) (*models.SubscriptionGroupLocalization, error) {
	params := url.Values{}
	params.Set("include", "subscriptionGroup")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/subscriptionGroupLocalizations/%s?%s", c.HostURL, localizationID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionGroupLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateSubscriptionGroupLocalization adds a localization to a group.
//
// The locale must be one the app itself supports, the same constraint a
// subscription localization is under.
func (c *Client) CreateSubscriptionGroupLocalization(groupID, locale, name string, customAppName *string, authToken *string) (*models.SubscriptionGroupLocalization, error) {
	requestData := models.Request[models.SubscriptionGroupLocalizationCreateRequest]{
		Data: models.SubscriptionGroupLocalizationCreateRequest{
			Type: "subscriptionGroupLocalizations",
			Attributes: models.SubscriptionGroupLocalizationCreateAttributes{
				Name:          name,
				Locale:        locale,
				CustomAppName: customAppName,
			},
			Relationships: models.SubscriptionGroupLocalizationCreateRelationships{
				SubscriptionGroup: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "subscriptionGroups", ID: groupID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/subscriptionGroupLocalizations", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionGroupLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateSubscriptionGroupLocalization updates a localization's name or custom
// app name.
//
// The locale is absent from Apple's update request: it identifies the
// localization, so changing it means replacing the record.
func (c *Client) UpdateSubscriptionGroupLocalization(localizationID string, name, customAppName *string, authToken *string) (*models.SubscriptionGroupLocalization, error) {
	requestData := models.Request[models.SubscriptionGroupLocalizationUpdateRequest]{
		Data: models.SubscriptionGroupLocalizationUpdateRequest{
			Type: "subscriptionGroupLocalizations",
			ID:   localizationID,
			Attributes: models.SubscriptionGroupLocalizationUpdateAttributes{
				Name:          name,
				CustomAppName: customAppName,
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/subscriptionGroupLocalizations/%s", c.HostURL, localizationID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionGroupLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteSubscriptionGroupLocalization deletes a localization.
func (c *Client) DeleteSubscriptionGroupLocalization(localizationID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/subscriptionGroupLocalizations/%s", c.HostURL, localizationID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// GetSubscriptionGroupLocalizationByLocale finds a group's localization for one
// locale. Apple permits only one per locale, so the match is unambiguous.
func (c *Client) GetSubscriptionGroupLocalizationByLocale(groupID, locale string) (*models.SubscriptionGroupLocalization, error) {
	localizations, err := c.GetSubscriptionGroupLocalizations(groupID)
	if err != nil {
		return nil, err
	}

	for _, localization := range localizations {
		if localization.Attributes.Locale == locale {
			return &localization, nil
		}
	}

	return nil, fmt.Errorf("localization for locale '%s' not found on subscription group '%s'", locale, groupID)
}
