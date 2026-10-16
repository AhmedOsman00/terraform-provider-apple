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

// GetAppScreenshotSets lists the screenshot sets of one App Store version
// localization -- one per display type that has been started.
func (c *Client) GetAppScreenshotSets(localizationID string) ([]models.AppScreenshotSet, error) {
	return getAllPages[models.AppScreenshotSet](c,
		fmt.Sprintf("/v1/appStoreVersionLocalizations/%s/appScreenshotSets", localizationID), noPageSize)
}

// GetAppScreenshotSetByDisplayType finds the set for one display type. A
// localization holds at most one per type.
func (c *Client) GetAppScreenshotSetByDisplayType(localizationID, displayType string) (*models.AppScreenshotSet, error) {
	sets, err := c.GetAppScreenshotSets(localizationID)
	if err != nil {
		return nil, err
	}

	for _, set := range sets {
		if set.Attributes.ScreenshotDisplayType == displayType {
			return &set, nil
		}
	}

	return nil, fmt.Errorf("screenshot set '%s' not found on localization '%s'", displayType, localizationID)
}

// GetAppScreenshotSet reads one set, including the localization it belongs to
// so an import can recover the parent.
func (c *Client) GetAppScreenshotSet(setID string) (*models.AppScreenshotSet, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s/v1/appScreenshotSets/%s?include=appStoreVersionLocalization", c.HostURL, setID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppScreenshotSet]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateAppScreenshotSet starts a set for one display type.
func (c *Client) CreateAppScreenshotSet(localizationID, displayType string, authToken *string) (*models.AppScreenshotSet, error) {
	requestData := models.Request[models.AppScreenshotSetCreateRequest]{
		Data: models.AppScreenshotSetCreateRequest{
			Type:       "appScreenshotSets",
			Attributes: models.AppScreenshotSetAttributes{ScreenshotDisplayType: displayType},
			Relationships: models.AppScreenshotSetCreateRelationships{
				AppStoreVersionLocalization: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "appStoreVersionLocalizations", ID: localizationID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/appScreenshotSets", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppScreenshotSet]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteAppScreenshotSet deletes a set together with every screenshot in it.
func (c *Client) DeleteAppScreenshotSet(setID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/appScreenshotSets/%s", c.HostURL, setID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// GetAppScreenshots lists a set's screenshots in display order.
func (c *Client) GetAppScreenshots(setID string) ([]models.MediaAsset, error) {
	return getAllPages[models.MediaAsset](c,
		fmt.Sprintf("/v1/appScreenshotSets/%s/appScreenshots", setID), noPageSize)
}

// UploadAppScreenshot uploads one file into a set, appending it to the end.
func (c *Client) UploadAppScreenshot(setID, fileName string, data []byte) (*models.MediaAsset, error) {
	return c.uploadMedia(appScreenshotKind, setID, fileName, data)
}

// DeleteAppScreenshot removes one screenshot from its set.
func (c *Client) DeleteAppScreenshot(screenshotID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/appScreenshots/%s", c.HostURL, screenshotID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// ReorderAppScreenshots sets the display order of a set's screenshots.
//
// The relationship PATCH replaces the whole ordered list, so ids must name
// every screenshot the set holds -- one left out is not removed, the request is
// refused.
func (c *Client) ReorderAppScreenshots(setID string, screenshotIDs []string, authToken *string) error {
	data := make([]models.ResourceData, 0, len(screenshotIDs))
	for _, id := range screenshotIDs {
		data = append(data, models.ResourceData{Type: "appScreenshots", ID: id})
	}

	rb, err := json.Marshal(models.ResourceIdentifiers{Data: data})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PATCH",
		fmt.Sprintf("%s/v1/appScreenshotSets/%s/relationships/appScreenshots", c.HostURL, setID),
		strings.NewReader(string(rb)))
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}
