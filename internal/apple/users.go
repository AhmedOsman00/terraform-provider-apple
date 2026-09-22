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

// GetUsers retrieves every member of the App Store Connect team.
func (c *Client) GetUsers() ([]models.User, error) {
	return getAllPages[models.User](c, "/v1/users", defaultPageSize)
}

// GetUser finds one team member by Apple's identifier.
//
// Apple publishes no GET /v1/users/{id} -- the collection is the only read
// there is -- so this scans it, the way the Developer Portal by-identifier
// helpers do. A team is tens of people rather than thousands, so the scan is
// one request in practice.
func (c *Client) GetUser(userID string) (*models.User, error) {
	users, err := c.GetUsers()
	if err != nil {
		return nil, err
	}

	for i := range users {
		if users[i].ID == userID {
			return &users[i], nil
		}
	}

	return nil, fmt.Errorf("user '%s' not found in this App Store Connect team", userID)
}

// GetUserByUsername finds one team member by the address they sign in with.
//
// filter[username] narrows the request server-side, but the match is made again
// in memory so a filter Apple ignores costs a larger page rather than the wrong
// person. Addresses are compared case-insensitively: Apple preserves the case
// the Apple Account was created with and matches without it.
func (c *Client) GetUserByUsername(username string) (*models.User, error) {
	params := url.Values{}
	params.Set("filter[username]", username)

	users, err := getAllPagesQuery[models.User](c, "/v1/users", defaultPageSize, params)
	if err != nil {
		return nil, err
	}

	for i := range users {
		if reported := users[i].Attributes.Username; reported != nil && strings.EqualFold(*reported, username) {
			return &users[i], nil
		}
	}

	return nil, fmt.Errorf("user '%s' not found in this App Store Connect team", username)
}

// GetUserVisibleApps retrieves the apps one team member can see.
//
// This is a paginated collection of its own rather than a field on the user, so
// reading a user's app visibility in full is a second request -- the same shape
// the availability records are in with their territories. For a user whose
// allAppsVisible is true Apple answers with every app on the team, which is why
// the provider only asks when the user is restricted to a list.
func (c *Client) GetUserVisibleApps(userID string) ([]models.App, error) {
	return getAllPages[models.App](c, fmt.Sprintf("/v1/users/%s/visibleApps", userID), defaultPageSize)
}

// UpdateUser changes a team member's roles, app visibility and provisioning
// access.
//
// visibleApps is sent only when the user is restricted to a list: a user with
// allAppsVisible true has no list, and Apple rejects a request carrying both.
// An empty, non-nil visibleApps is a user who can see no apps at all, which is
// a real answer and has to survive the round trip -- hence an explicit [] here
// rather than an omitted member.
func (c *Client) UpdateUser(userID string, attributes models.UserUpdateAttributes, visibleAppIDs []string, authToken *string) (*models.User, error) {
	updateRequest := models.UserUpdateRequest{
		Type:       "users",
		ID:         userID,
		Attributes: attributes,
	}

	if visibleAppIDs != nil {
		data := make([]models.ResourceData, 0, len(visibleAppIDs))
		for _, appID := range visibleAppIDs {
			data = append(data, models.ResourceData{Type: "apps", ID: appID})
		}

		updateRequest.Relationships = &models.UserUpdateRelationships{
			VisibleApps: models.ResourceIdentifiers{Data: data},
		}
	}

	requestData := models.Request[models.UserUpdateRequest]{Data: updateRequest}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/users/%s", c.HostURL, userID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.User]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteUser removes a person from the App Store Connect team.
//
// This revokes their access to every app on the team. It is not reversible by
// the API: the person has to be invited again and has to accept again. Apple
// refuses to remove the account holder.
func (c *Client) DeleteUser(userID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/users/%s", c.HostURL, userID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
