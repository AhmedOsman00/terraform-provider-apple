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

// GetSubscriptionLocalizations retrieves every localization of a subscription.
func (c *Client) GetSubscriptionLocalizations(subscriptionID string) ([]models.SubscriptionLocalization, error) {
	return getAllPages[models.SubscriptionLocalization](
		c,
		fmt.Sprintf("/v1/subscriptions/%s/subscriptionLocalizations", subscriptionID),
		defaultPageSize,
	)
}

// GetSubscriptionLocalization retrieves a specific localization by its ID.
//
// include=subscription populates the parent linkage, which import needs to
// write complete state from a bare localization ID.
func (c *Client) GetSubscriptionLocalization(localizationID string) (*models.SubscriptionLocalization, error) {
	params := url.Values{}
	params.Set("include", "subscription")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/subscriptionLocalizations/%s?%s", c.HostURL, localizationID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateSubscriptionLocalization adds a localization to a subscription.
//
// A subscription needs at least one before Apple will move it out of
// MISSING_METADATA. The locale must be one the app itself supports.
func (c *Client) CreateSubscriptionLocalization(subscriptionID, locale, name string, description *string, authToken *string) (*models.SubscriptionLocalization, error) {
	requestData := models.Request[models.SubscriptionLocalizationCreateRequest]{
		Data: models.SubscriptionLocalizationCreateRequest{
			Type: "subscriptionLocalizations",
			Attributes: models.SubscriptionLocalizationCreateAttributes{
				Name:        name,
				Locale:      locale,
				Description: description,
			},
			Relationships: models.SubscriptionLocalizationCreateRelationships{
				Subscription: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "subscriptions", ID: subscriptionID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/subscriptionLocalizations", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateSubscriptionLocalization updates a localization's name or description.
//
// The locale is absent from Apple's update request: it identifies the
// localization, so changing it means replacing the record.
func (c *Client) UpdateSubscriptionLocalization(localizationID string, name, description *string, authToken *string) (*models.SubscriptionLocalization, error) {
	requestData := models.Request[models.SubscriptionLocalizationUpdateRequest]{
		Data: models.SubscriptionLocalizationUpdateRequest{
			Type: "subscriptionLocalizations",
			ID:   localizationID,
			Attributes: models.SubscriptionLocalizationUpdateAttributes{
				Name:        name,
				Description: description,
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/subscriptionLocalizations/%s", c.HostURL, localizationID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteSubscriptionLocalization deletes a localization.
func (c *Client) DeleteSubscriptionLocalization(localizationID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/subscriptionLocalizations/%s", c.HostURL, localizationID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// GetSubscriptionLocalizationByLocale finds a subscription's localization for
// one locale. Apple permits only one per locale, so the match is unambiguous.
func (c *Client) GetSubscriptionLocalizationByLocale(subscriptionID, locale string) (*models.SubscriptionLocalization, error) {
	localizations, err := c.GetSubscriptionLocalizations(subscriptionID)
	if err != nil {
		return nil, err
	}

	for _, localization := range localizations {
		if localization.Attributes.Locale == locale {
			return &localization, nil
		}
	}

	return nil, fmt.Errorf("localization for locale '%s' not found on subscription '%s'", locale, subscriptionID)
}
