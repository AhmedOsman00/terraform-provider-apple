// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// Submitting for review is the modern three-step flow: create a submission for
// the app and platform, add the version to it as an item, then mark the
// submission submitted. The older appStoreVersionSubmissions endpoint is gone.
//
// It is an event rather than state, which is why the provider has no resource
// for it and the CLI does it instead.

// GetReviewSubmissions lists an app's submissions on one platform in the given
// states, filtered server-side.
func (c *Client) GetReviewSubmissions(appID, platform string, states []string) ([]models.ReviewSubmission, error) {
	params := url.Values{}
	if platform != "" {
		params.Set("filter[platform]", platform)
	}
	if len(states) > 0 {
		params.Set("filter[state]", strings.Join(states, ","))
	}

	return getAllPagesQuery[models.ReviewSubmission](c, fmt.Sprintf("/v1/apps/%s/reviewSubmissions", appID), defaultPageSize, params)
}

// CreateReviewSubmission opens a draft submission. Apple refuses a second
// while one is open for the same platform, so callers reuse an open one first.
func (c *Client) CreateReviewSubmission(appID, platform string) (*models.ReviewSubmission, error) {
	requestData := models.Request[models.ReviewSubmissionCreateRequest]{
		Data: models.ReviewSubmissionCreateRequest{
			Type:       "reviewSubmissions",
			Attributes: models.ReviewSubmissionAttributes{Platform: platform},
			Relationships: map[string]models.ResourceIdentifier{
				"app": {Data: models.ResourceData{Type: "apps", ID: appID}},
			},
		},
	}

	response := models.Response[models.ReviewSubmission]{}
	if err := c.sendJSON("POST", "/v1/reviewSubmissions", requestData, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetReviewSubmissionItems lists what a submission holds, with each item's
// version, which Apple reports only when it is asked to include it.
func (c *Client) GetReviewSubmissionItems(submissionID string) ([]models.ReviewSubmissionItem, error) {
	params := url.Values{}
	params.Set("include", "appStoreVersion")

	return getAllPagesQuery[models.ReviewSubmissionItem](c, fmt.Sprintf("/v1/reviewSubmissions/%s/items", submissionID), defaultPageSize, params)
}

// AddVersionToReviewSubmission puts a version in a draft submission. This is
// the request Apple checks the version on: anything missing -- a build,
// screenshots, an answer App Store Connect requires -- is refused here, one
// error per problem.
func (c *Client) AddVersionToReviewSubmission(submissionID, versionID string) (*models.ReviewSubmissionItem, error) {
	requestData := models.Request[models.ReviewSubmissionItemCreateRequest]{
		Data: models.ReviewSubmissionItemCreateRequest{
			Type: "reviewSubmissionItems",
			Relationships: map[string]models.ResourceIdentifier{
				"reviewSubmission": {Data: models.ResourceData{Type: "reviewSubmissions", ID: submissionID}},
				"appStoreVersion":  {Data: models.ResourceData{Type: "appStoreVersions", ID: versionID}},
			},
		},
	}

	response := models.Response[models.ReviewSubmissionItem]{}
	if err := c.sendJSON("POST", "/v1/reviewSubmissionItems", requestData, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// SubmitReviewSubmission sends a draft submission, or resubmits one App Review
// left with unresolved issues.
func (c *Client) SubmitReviewSubmission(submissionID string) (*models.ReviewSubmission, error) {
	requestData := models.Request[models.ReviewSubmissionSubmitRequest]{
		Data: models.ReviewSubmissionSubmitRequest{
			Type:       "reviewSubmissions",
			ID:         submissionID,
			Attributes: models.ReviewSubmissionSubmitAttributes{Submitted: true},
		},
	}

	response := models.Response[models.ReviewSubmission]{}
	if err := c.sendJSON("PATCH", "/v1/reviewSubmissions/"+submissionID, requestData, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}
