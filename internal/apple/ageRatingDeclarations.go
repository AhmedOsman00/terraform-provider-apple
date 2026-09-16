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

// GetAgeRatingDeclaration reads the answered content questionnaire behind an
// AppInfo's age rating.
//
// Apple creates the record with the app and publishes neither POST nor DELETE
// for it, so it is only ever read and patched. It hangs off the AppInfo, not
// the app and not the version: the rating describes the app.
func (c *Client) GetAgeRatingDeclaration(appInfoID string) (*models.AgeRatingDeclaration, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/appInfos/%s/ageRatingDeclaration", c.HostURL, appInfoID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AgeRatingDeclaration]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Data.ID == "" {
		return nil, fmt.Errorf("age rating declaration not found for app info '%s'", appInfoID)
	}

	return &response.Data, nil
}

// UpdateAgeRatingDeclaration answers the content questionnaire.
//
// Only the attributes present in the request are touched; an omitted answer
// leaves Apple's existing value alone, which is not the same as answering NONE.
func (c *Client) UpdateAgeRatingDeclaration(declarationID string, attributes models.AgeRatingDeclarationAttributes, authToken *string) (*models.AgeRatingDeclaration, error) {
	requestData := models.Request[models.AgeRatingDeclarationUpdateRequest]{
		Data: models.AgeRatingDeclarationUpdateRequest{
			Type:       "ageRatingDeclarations",
			ID:         declarationID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/ageRatingDeclarations/%s", c.HostURL, declarationID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AgeRatingDeclaration]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}
