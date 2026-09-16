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

// GetAppInfoLocalizations retrieves every localization on an AppInfo.
func (c *Client) GetAppInfoLocalizations(appInfoID string) ([]models.AppInfoLocalization, error) {
	return getAllPages[models.AppInfoLocalization](
		c,
		fmt.Sprintf("/v1/appInfos/%s/appInfoLocalizations", appInfoID),
		defaultPageSize,
	)
}

// GetAppInfoLocalization retrieves a single localization by its ID.
//
// include=appInfo populates the parent linkage, which is what lets an import
// accept a bare localization ID: the app itself is then one further hop, since
// an AppInfo does report the app it belongs to.
func (c *Client) GetAppInfoLocalization(localizationID string) (*models.AppInfoLocalization, error) {
	params := url.Values{}
	params.Set("include", "appInfo")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/appInfoLocalizations/%s?%s", c.HostURL, localizationID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppInfoLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetAppInfoLocalizationByLocale finds a localization on an AppInfo by locale.
//
// Apple publishes no filter for this, so the collection is listed and scanned.
// The locale is the natural key -- an AppInfo holds at most one localization
// per language.
func (c *Client) GetAppInfoLocalizationByLocale(appInfoID, locale string) (*models.AppInfoLocalization, error) {
	localizations, err := c.GetAppInfoLocalizations(appInfoID)
	if err != nil {
		return nil, err
	}

	for i := range localizations {
		if localizations[i].Attributes.Locale == locale {
			return &localizations[i], nil
		}
	}

	return nil, fmt.Errorf("app info localization for locale '%s' not found on app info '%s'", locale, appInfoID)
}

// CreateAppInfoLocalization adds a localized app name to an AppInfo.
func (c *Client) CreateAppInfoLocalization(appInfoID string, attributes models.AppInfoLocalizationCreateAttributes, authToken *string) (*models.AppInfoLocalization, error) {
	requestData := models.Request[models.AppInfoLocalizationCreateRequest]{
		Data: models.AppInfoLocalizationCreateRequest{
			Type:       "appInfoLocalizations",
			Attributes: attributes,
			Relationships: models.AppInfoLocalizationCreateRelationships{
				AppInfo: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "appInfos", ID: appInfoID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/appInfoLocalizations", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppInfoLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateAppInfoLocalization updates a localized app name.
//
// The locale is absent from Apple's update request: it identifies the record.
func (c *Client) UpdateAppInfoLocalization(localizationID string, attributes models.AppInfoLocalizationUpdateAttributes, authToken *string) (*models.AppInfoLocalization, error) {
	requestData := models.Request[models.AppInfoLocalizationUpdateRequest]{
		Data: models.AppInfoLocalizationUpdateRequest{
			Type:       "appInfoLocalizations",
			ID:         localizationID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/appInfoLocalizations/%s", c.HostURL, localizationID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppInfoLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteAppInfoLocalization removes a localized app name.
//
// Apple refuses to delete the localization for the app's primary locale: every
// app must have a name in the language it was created in.
func (c *Client) DeleteAppInfoLocalization(localizationID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/appInfoLocalizations/%s", c.HostURL, localizationID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
