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

// GetBetaBuildLocalizations retrieves a build's "What to Test" notes in every
// language they have been written in.
func (c *Client) GetBetaBuildLocalizations(buildID string) ([]models.BetaBuildLocalization, error) {
	return getAllPages[models.BetaBuildLocalization](c, fmt.Sprintf("/v1/builds/%s/betaBuildLocalizations", buildID), defaultPageSize)
}

// GetBetaBuildLocalization retrieves one note by its ID, carrying the build it
// belongs to.
func (c *Client) GetBetaBuildLocalization(localizationID string) (*models.BetaBuildLocalization, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/betaBuildLocalizations/%s?include=build", c.HostURL, localizationID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaBuildLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetBetaBuildLocalizationByLocale finds a build's note for one language.
//
// Apple creates a note for the app's primary locale on upload, so the record
// being looked for often already exists -- which is why the resource that uses
// this adopts rather than posting blindly.
func (c *Client) GetBetaBuildLocalizationByLocale(buildID, locale string) (*models.BetaBuildLocalization, error) {
	localizations, err := c.GetBetaBuildLocalizations(buildID)
	if err != nil {
		return nil, err
	}

	for i := range localizations {
		if localizations[i].Attributes.Locale == locale {
			return &localizations[i], nil
		}
	}

	return nil, fmt.Errorf("beta build localization for locale '%s' not found on build '%s'", locale, buildID)
}

// CreateBetaBuildLocalization writes a build's "What to Test" note for one
// language.
func (c *Client) CreateBetaBuildLocalization(buildID string, attributes models.BetaBuildLocalizationCreateAttributes, authToken *string) (*models.BetaBuildLocalization, error) {
	requestData := models.Request[models.BetaBuildLocalizationCreateRequest]{
		Data: models.BetaBuildLocalizationCreateRequest{
			Type:       "betaBuildLocalizations",
			Attributes: attributes,
			Relationships: models.BetaBuildLocalizationCreateRelationships{
				Build: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "builds", ID: buildID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/betaBuildLocalizations", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaBuildLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateBetaBuildLocalization updates the note in place. whatsNew is the only
// member of Apple's update request: locale identifies the record and the build
// is fixed.
func (c *Client) UpdateBetaBuildLocalization(localizationID string, whatsNew *string, authToken *string) (*models.BetaBuildLocalization, error) {
	requestData := models.Request[models.BetaBuildLocalizationUpdateRequest]{
		Data: models.BetaBuildLocalizationUpdateRequest{
			Type: "betaBuildLocalizations",
			ID:   localizationID,
			Attributes: models.BetaBuildLocalizationUpdateAttributes{
				WhatsNew: whatsNew,
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/betaBuildLocalizations/%s", c.HostURL, localizationID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaBuildLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteBetaBuildLocalization removes one language's note from a build.
func (c *Client) DeleteBetaBuildLocalization(localizationID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/betaBuildLocalizations/%s", c.HostURL, localizationID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
