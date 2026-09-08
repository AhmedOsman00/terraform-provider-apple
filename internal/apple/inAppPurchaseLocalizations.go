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

// GetInAppPurchaseLocalizations retrieves every localization of one version.
//
// The collection hangs off the version rather than the purchase: the v1
// endpoints that pretended otherwise are deprecated as of App Store Connect API
// 4.4.1.
func (c *Client) GetInAppPurchaseLocalizations(versionID string) ([]models.InAppPurchaseLocalization, error) {
	return getAllPages[models.InAppPurchaseLocalization](
		c,
		fmt.Sprintf("/v1/inAppPurchaseVersions/%s/localizations", versionID),
		defaultPageSize,
	)
}

// GetInAppPurchaseLocalization retrieves a specific localization by its ID.
//
// include=version populates the parent linkage. That is one hop short of the
// purchase -- GetInAppPurchaseLocalizationPurchaseID makes the second.
func (c *Client) GetInAppPurchaseLocalization(localizationID string) (*models.InAppPurchaseLocalization, error) {
	params := url.Values{}
	params.Set("include", "version")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/inAppPurchaseLocalizations/%s?%s", c.HostURL, localizationID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchaseLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetInAppPurchaseLocalizationPurchaseID resolves the purchase a localization
// belongs to, through the version that owns it.
//
// It takes two requests because Apple models the ownership in two steps, and
// there is no include chain that collapses them.
func (c *Client) GetInAppPurchaseLocalizationPurchaseID(localization *models.InAppPurchaseLocalization) (string, error) {
	if localization.Relationships == nil || localization.Relationships.Version == nil {
		return "", fmt.Errorf("localization '%s' reports no version, so its in-app purchase cannot be resolved", localization.ID)
	}

	versionID := localization.Relationships.Version.Data.ID
	if versionID == "" {
		return "", fmt.Errorf("localization '%s' reports an empty version linkage", localization.ID)
	}

	version, err := c.GetInAppPurchaseVersion(versionID)
	if err != nil {
		return "", err
	}

	if version.Relationships == nil || version.Relationships.InAppPurchase == nil {
		return "", fmt.Errorf("version '%s' reports no in-app purchase", versionID)
	}

	return version.Relationships.InAppPurchase.Data.ID, nil
}

// CreateInAppPurchaseLocalization adds a localization to a version.
//
// The locale must be one the app itself supports; Apple rejects a localization
// for a locale the app has not been localized into.
func (c *Client) CreateInAppPurchaseLocalization(versionID, locale, name string, description *string, authToken *string) (*models.InAppPurchaseLocalization, error) {
	requestData := models.Request[models.InAppPurchaseLocalizationCreateRequest]{
		Data: models.InAppPurchaseLocalizationCreateRequest{
			Type: "inAppPurchaseLocalizations",
			Attributes: models.InAppPurchaseLocalizationCreateAttributes{
				Name:        name,
				Locale:      locale,
				Description: description,
			},
			Relationships: models.InAppPurchaseLocalizationCreateRelationships{
				Version: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "inAppPurchaseVersions", ID: versionID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v2/inAppPurchaseLocalizations", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchaseLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateInAppPurchaseLocalization updates a localization's name or description.
//
// The locale is absent from Apple's update request: it identifies the record,
// so changing it means replacing it.
func (c *Client) UpdateInAppPurchaseLocalization(localizationID string, name, description *string, authToken *string) (*models.InAppPurchaseLocalization, error) {
	requestData := models.Request[models.InAppPurchaseLocalizationUpdateRequest]{
		Data: models.InAppPurchaseLocalizationUpdateRequest{
			Type: "inAppPurchaseLocalizations",
			ID:   localizationID,
			Attributes: models.InAppPurchaseLocalizationUpdateAttributes{
				Name:        name,
				Description: description,
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v2/inAppPurchaseLocalizations/%s", c.HostURL, localizationID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchaseLocalization]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteInAppPurchaseLocalization deletes a localization.
func (c *Client) DeleteInAppPurchaseLocalization(localizationID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v2/inAppPurchaseLocalizations/%s", c.HostURL, localizationID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// GetInAppPurchaseLocalizationByLocale finds a version's localization for one
// locale. Apple permits only one per locale, so the match is unambiguous.
func (c *Client) GetInAppPurchaseLocalizationByLocale(versionID, locale string) (*models.InAppPurchaseLocalization, error) {
	localizations, err := c.GetInAppPurchaseLocalizations(versionID)
	if err != nil {
		return nil, err
	}

	for _, localization := range localizations {
		if localization.Attributes.Locale == locale {
			return &localization, nil
		}
	}

	return nil, fmt.Errorf("localization for locale '%s' not found on in-app purchase version '%s'", locale, versionID)
}
