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

// GetBetaGroups retrieves every TestFlight group belonging to an app.
//
// Apple does publish a top-level /v1/betaGroups collection, but reading it
// through the app keeps the ownership check the rest of the App Store Connect
// resources in this provider rely on: a group found here demonstrably belongs
// to the app that was asked about.
func (c *Client) GetBetaGroups(appID string) ([]models.BetaGroup, error) {
	return getAllPages[models.BetaGroup](c, fmt.Sprintf("/v1/apps/%s/betaGroups", appID), defaultPageSize)
}

// GetBetaGroup retrieves a specific group by its ID.
//
// Unlike a subscription group, whose owning app can never be recovered, a beta
// group reports one: "app" is among the values GET /v1/betaGroups/{id} accepts
// for include, so a bare ID is enough to import from.
func (c *Client) GetBetaGroup(groupID string) (*models.BetaGroup, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/betaGroups/%s?include=app", c.HostURL, groupID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaGroup]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetBetaGroupByName finds a group by name within one app.
//
// Apple enforces the name unique per app -- a second group by the same name is
// refused -- so the first match is the only match.
func (c *Client) GetBetaGroupByName(appID, name string) (*models.BetaGroup, error) {
	groups, err := c.GetBetaGroups(appID)
	if err != nil {
		return nil, err
	}

	for i := range groups {
		if groups[i].Attributes.Name != nil && *groups[i].Attributes.Name == name {
			return &groups[i], nil
		}
	}

	return nil, fmt.Errorf("beta group '%s' not found in app '%s'", name, appID)
}

// CreateBetaGroup creates a TestFlight group under an app.
func (c *Client) CreateBetaGroup(appID string, attributes models.BetaGroupCreateAttributes, authToken *string) (*models.BetaGroup, error) {
	requestData := models.Request[models.BetaGroupCreateRequest]{
		Data: models.BetaGroupCreateRequest{
			Type:       "betaGroups",
			Attributes: attributes,
			Relationships: models.BetaGroupCreateRelationships{
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

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/betaGroups", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaGroup]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateBetaGroup updates the attributes Apple's update request accepts.
//
// isInternalGroup and hasAccessToAllBuilds are not among them: a group does not
// change kind, and the set of builds an internal group sees is fixed at
// creation.
func (c *Client) UpdateBetaGroup(groupID string, attributes models.BetaGroupUpdateAttributes, authToken *string) (*models.BetaGroup, error) {
	requestData := models.Request[models.BetaGroupUpdateRequest]{
		Data: models.BetaGroupUpdateRequest{
			Type:       "betaGroups",
			ID:         groupID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/betaGroups/%s", c.HostURL, groupID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaGroup]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteBetaGroup deletes a TestFlight group.
//
// Deleting a group removes its testers' access to the app's builds; it does not
// delete the testers themselves, who remain in the app's tester list and in any
// other group they belong to.
func (c *Client) DeleteBetaGroup(groupID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/betaGroups/%s", c.HostURL, groupID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
