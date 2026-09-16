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

// GetAppStoreReviewDetail reads the review contact and demo account of a
// version.
//
// A version has exactly one review detail and Apple publishes no collection of
// them, so it is reachable only through the version. A version that has never
// had one answers 200 with a null data member rather than 404, which is
// reported here as not found so callers can treat it like any other missing
// resource -- and, in this case, create one.
func (c *Client) GetAppStoreReviewDetail(versionID string) (*models.AppStoreReviewDetail, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/appStoreVersions/%s/appStoreReviewDetail", c.HostURL, versionID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreReviewDetail]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Data.ID == "" {
		return nil, fmt.Errorf("app store review detail not found for version '%s'", versionID)
	}

	return &response.Data, nil
}

// CreateAppStoreReviewDetail attaches review information to a version.
func (c *Client) CreateAppStoreReviewDetail(versionID string, attributes models.AppStoreReviewDetailAttributes, authToken *string) (*models.AppStoreReviewDetail, error) {
	requestData := models.Request[models.AppStoreReviewDetailCreateRequest]{
		Data: models.AppStoreReviewDetailCreateRequest{
			Type:       "appStoreReviewDetails",
			Attributes: attributes,
			Relationships: models.AppStoreReviewDetailCreateRelationships{
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

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/appStoreReviewDetails", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreReviewDetail]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateAppStoreReviewDetail updates review information in place.
func (c *Client) UpdateAppStoreReviewDetail(detailID string, attributes models.AppStoreReviewDetailAttributes, authToken *string) (*models.AppStoreReviewDetail, error) {
	requestData := models.Request[models.AppStoreReviewDetailUpdateRequest]{
		Data: models.AppStoreReviewDetailUpdateRequest{
			Type:       "appStoreReviewDetails",
			ID:         detailID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/appStoreReviewDetails/%s", c.HostURL, detailID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppStoreReviewDetail]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}
