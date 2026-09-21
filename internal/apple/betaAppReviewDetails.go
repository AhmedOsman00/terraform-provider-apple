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

// GetBetaAppReviewDetail reads what the TestFlight beta reviewers are told
// about an app.
//
// An app has exactly one, reachable only through the app -- Apple publishes no
// collection of them. The record is created with the app, so a missing one is
// an anomaly rather than the ordinary "not created yet" this shape usually
// means; it is still reported as not found, because the caller has no more to
// go on either way. A to-one relationship with no value answers 200 with a null
// data member rather than 404, which is why the empty ID is checked.
func (c *Client) GetBetaAppReviewDetail(appID string) (*models.BetaAppReviewDetail, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/apps/%s/betaAppReviewDetail", c.HostURL, appID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaAppReviewDetail]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Data.ID == "" {
		return nil, fmt.Errorf("beta app review detail not found for app '%s'", appID)
	}

	return &response.Data, nil
}

// UpdateBetaAppReviewDetail updates the beta review information in place.
//
// There is no create: Apple publishes no POST /v1/betaAppReviewDetails, because
// the record comes into existence with the app.
func (c *Client) UpdateBetaAppReviewDetail(detailID string, attributes models.BetaAppReviewDetailAttributes, authToken *string) (*models.BetaAppReviewDetail, error) {
	requestData := models.Request[models.BetaAppReviewDetailUpdateRequest]{
		Data: models.BetaAppReviewDetailUpdateRequest{
			Type:       "betaAppReviewDetails",
			ID:         detailID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/betaAppReviewDetails/%s", c.HostURL, detailID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaAppReviewDetail]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}
