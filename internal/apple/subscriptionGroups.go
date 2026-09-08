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

// GetSubscriptionGroups retrieves every subscription group belonging to an app.
//
// Apple publishes no top-level /v1/subscriptionGroups collection, so a group
// can only be listed through the app that owns it.
func (c *Client) GetSubscriptionGroups(appID string) ([]models.SubscriptionGroup, error) {
	return getAllPages[models.SubscriptionGroup](c, fmt.Sprintf("/v1/apps/%s/subscriptionGroups", appID), defaultPageSize)
}

// GetSubscriptionGroup retrieves a specific subscription group by its ID.
//
// The response carries no app linkage: "app" is not among the values GET
// /v1/subscriptionGroups/{id} accepts for its include parameter, so the owning
// app cannot be recovered from the group alone.
func (c *Client) GetSubscriptionGroup(groupID string) (*models.SubscriptionGroup, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/subscriptionGroups/%s", c.HostURL, groupID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionGroup]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateSubscriptionGroup creates a subscription group under an app.
func (c *Client) CreateSubscriptionGroup(appID, referenceName string, authToken *string) (*models.SubscriptionGroup, error) {
	requestData := models.Request[models.SubscriptionGroupCreateRequest]{
		Data: models.SubscriptionGroupCreateRequest{
			Type: "subscriptionGroups",
			Attributes: models.SubscriptionGroupAttributes{
				ReferenceName: referenceName,
			},
			Relationships: models.SubscriptionGroupCreateRelationships{
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

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/subscriptionGroups", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionGroup]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateSubscriptionGroup updates a group's reference name, the only attribute
// Apple's update request accepts.
func (c *Client) UpdateSubscriptionGroup(groupID, referenceName string, authToken *string) (*models.SubscriptionGroup, error) {
	requestData := models.Request[models.SubscriptionGroupUpdateRequest]{
		Data: models.SubscriptionGroupUpdateRequest{
			Type: "subscriptionGroups",
			ID:   groupID,
			Attributes: models.SubscriptionGroupAttributes{
				ReferenceName: referenceName,
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/subscriptionGroups/%s", c.HostURL, groupID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionGroup]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteSubscriptionGroup deletes a subscription group.
//
// Apple refuses to delete a group that still holds a subscription which has
// ever been approved, so a destroy can fail for reasons Terraform cannot
// resolve on its own.
func (c *Client) DeleteSubscriptionGroup(groupID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/subscriptionGroups/%s", c.HostURL, groupID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// GetSubscriptionGroupByReferenceName finds a group by its reference name
// within one app. Reference names are not enforced unique by Apple, so this
// returns the first match.
func (c *Client) GetSubscriptionGroupByReferenceName(appID, referenceName string) (*models.SubscriptionGroup, error) {
	groups, err := c.GetSubscriptionGroups(appID)
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		if group.Attributes.ReferenceName == referenceName {
			return &group, nil
		}
	}

	return nil, fmt.Errorf("subscription group with reference name '%s' not found in app '%s'", referenceName, appID)
}
