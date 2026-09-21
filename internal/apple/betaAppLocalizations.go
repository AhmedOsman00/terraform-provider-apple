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

// GetBetaAppLocalizations retrieves an app's TestFlight page text in every
// language it has been written in.
func (c *Client) GetBetaAppLocalizations(appID string) ([]models.BetaAppLocalization, error) {
	return getAllPages[models.BetaAppLocalization](c, fmt.Sprintf("/v1/apps/%s/betaAppLocalizations", appID), defaultPageSize)
}

// GetBetaAppLocalization retrieves one localization by its ID, carrying the
// owning app so an import from a bare ID can write app_id.
func (c *Client) GetBetaAppLocalization(localizationID string) (*models.BetaAppLocalization, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/betaAppLocalizations/%s?include=app", c.HostURL, localizationID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaAppLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetBetaAppLocalizationByLocale finds an app's localization for one language.
//
// An app holds at most one per locale, so this is a lookup rather than a scan
// that could match twice.
func (c *Client) GetBetaAppLocalizationByLocale(appID, locale string) (*models.BetaAppLocalization, error) {
	localizations, err := c.GetBetaAppLocalizations(appID)
	if err != nil {
		return nil, err
	}

	for i := range localizations {
		if localizations[i].Attributes.Locale == locale {
			return &localizations[i], nil
		}
	}

	return nil, fmt.Errorf("beta app localization for locale '%s' not found in app '%s'", locale, appID)
}

// CreateBetaAppLocalization writes an app's TestFlight page text for one
// language.
func (c *Client) CreateBetaAppLocalization(appID string, attributes models.BetaAppLocalizationCreateAttributes, authToken *string) (*models.BetaAppLocalization, error) {
	requestData := models.Request[models.BetaAppLocalizationCreateRequest]{
		Data: models.BetaAppLocalizationCreateRequest{
			Type:       "betaAppLocalizations",
			Attributes: attributes,
			Relationships: models.BetaAppLocalizationCreateRelationships{
				App: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "apps", ID: appID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/betaAppLocalizations", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaAppLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateBetaAppLocalization updates the text in place. locale is absent from
// Apple's update request -- it identifies the record.
func (c *Client) UpdateBetaAppLocalization(localizationID string, attributes models.BetaAppLocalizationUpdateAttributes, authToken *string) (*models.BetaAppLocalization, error) {
	requestData := models.Request[models.BetaAppLocalizationUpdateRequest]{
		Data: models.BetaAppLocalizationUpdateRequest{
			Type:       "betaAppLocalizations",
			ID:         localizationID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/betaAppLocalizations/%s", c.HostURL, localizationID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaAppLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteBetaAppLocalization removes one language's TestFlight page text.
func (c *Client) DeleteBetaAppLocalization(localizationID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/betaAppLocalizations/%s", c.HostURL, localizationID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
