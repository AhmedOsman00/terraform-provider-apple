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

// GetAppStoreVersionLocalizations retrieves every localization on a version.
func (c *Client) GetAppStoreVersionLocalizations(versionID string) ([]models.AppStoreVersionLocalization, error) {
	return getAllPages[models.AppStoreVersionLocalization](
		c,
		fmt.Sprintf("/v1/appStoreVersions/%s/appStoreVersionLocalizations", versionID),
		defaultPageSize,
	)
}

// GetAppStoreVersionLocalization retrieves a single localization by its ID.
//
// include=appStoreVersion populates the parent linkage so an import can accept
// a bare ID.
func (c *Client) GetAppStoreVersionLocalization(localizationID string) (*models.AppStoreVersionLocalization, error) {
	params := url.Values{}
	params.Set("include", "appStoreVersion")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/appStoreVersionLocalizations/%s?%s", c.HostURL, localizationID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreVersionLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetAppStoreVersionLocalizationByLocale finds a version localization by
// locale.
//
// Apple publishes no filter for this, so the collection is listed and scanned.
func (c *Client) GetAppStoreVersionLocalizationByLocale(versionID, locale string) (*models.AppStoreVersionLocalization, error) {
	localizations, err := c.GetAppStoreVersionLocalizations(versionID)
	if err != nil {
		return nil, err
	}

	for i := range localizations {
		if localizations[i].Attributes.Locale == locale {
			return &localizations[i], nil
		}
	}

	return nil, fmt.Errorf("app store version localization for locale '%s' not found on version '%s'", locale, versionID)
}

// CreateAppStoreVersionLocalization adds a localized product page to a version.
func (c *Client) CreateAppStoreVersionLocalization(versionID string, attributes models.AppStoreVersionLocalizationCreateAttributes, authToken *string) (*models.AppStoreVersionLocalization, error) {
	requestData := models.Request[models.AppStoreVersionLocalizationCreateRequest]{
		Data: models.AppStoreVersionLocalizationCreateRequest{
			Type:       "appStoreVersionLocalizations",
			Attributes: attributes,
			Relationships: models.AppStoreVersionLocalizationCreateRelationships{
				AppStoreVersion: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "appStoreVersions", ID: versionID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/appStoreVersionLocalizations", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreVersionLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateAppStoreVersionLocalization updates a localized product page.
//
// The locale is absent from Apple's update request: it identifies the record.
func (c *Client) UpdateAppStoreVersionLocalization(localizationID string, attributes models.AppStoreVersionLocalizationUpdateAttributes, authToken *string) (*models.AppStoreVersionLocalization, error) {
	requestData := models.Request[models.AppStoreVersionLocalizationUpdateRequest]{
		Data: models.AppStoreVersionLocalizationUpdateRequest{
			Type:       "appStoreVersionLocalizations",
			ID:         localizationID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/appStoreVersionLocalizations/%s", c.HostURL, localizationID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreVersionLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteAppStoreVersionLocalization removes a localized product page.
//
// Apple refuses to remove the one for the app's primary locale: a version must
// keep at least the language it is sold in.
func (c *Client) DeleteAppStoreVersionLocalization(localizationID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/appStoreVersionLocalizations/%s", c.HostURL, localizationID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
