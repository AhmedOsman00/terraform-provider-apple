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

// GetUserInvitations retrieves every invitation still awaiting an answer.
//
// Accepted invitations are not here: Apple destroys the record and issues a
// User in its place, so this collection is only ever the pending ones.
func (c *Client) GetUserInvitations() ([]models.UserInvitation, error) {
	return getAllPages[models.UserInvitation](c, "/v1/userInvitations", defaultPageSize)
}

// GetUserInvitation retrieves a specific pending invitation by its ID.
//
// Unlike a user, an invitation does have a single-instance read. A 404 from it
// means the invitation is no longer pending -- accepted, cancelled or expired,
// and it cannot say which.
func (c *Client) GetUserInvitation(invitationID string) (*models.UserInvitation, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/userInvitations/%s", c.HostURL, invitationID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.UserInvitation]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetUserInvitationByEmail finds a pending invitation by the address it was
// sent to.
//
// filter[email] narrows the request server-side and the match is made again in
// memory, for the reason GetUserByUsername does it.
func (c *Client) GetUserInvitationByEmail(email string) (*models.UserInvitation, error) {
	params := url.Values{}
	params.Set("filter[email]", email)

	invitations, err := getAllPagesQuery[models.UserInvitation](c, "/v1/userInvitations", defaultPageSize, params)
	if err != nil {
		return nil, err
	}

	for i := range invitations {
		if reported := invitations[i].Attributes.Email; reported != nil && strings.EqualFold(*reported, email) {
			return &invitations[i], nil
		}
	}

	return nil, fmt.Errorf("no pending user invitation for '%s' in this App Store Connect team", email)
}

// GetUserInvitationVisibleApps retrieves the apps a pending invitation grants
// sight of.
//
// A separate paginated collection, like a user's -- see GetUserVisibleApps.
func (c *Client) GetUserInvitationVisibleApps(invitationID string) ([]models.App, error) {
	return getAllPages[models.App](c, fmt.Sprintf("/v1/userInvitations/%s/visibleApps", invitationID), defaultPageSize)
}

// CreateUserInvitation invites somebody to join the App Store Connect team.
//
// Apple emails the address immediately. The invitation lapses after 72 hours
// and, if accepted, is replaced by a User record with a different identifier.
//
// firstName and lastName are required by Apple here and nowhere else: they name
// the person in the invitation email and in the team list until their Apple
// Account supplies its own.
func (c *Client) CreateUserInvitation(attributes models.UserInvitationCreateAttributes, visibleAppIDs []string, authToken *string) (*models.UserInvitation, error) {
	createRequest := models.UserInvitationCreateRequest{
		Type:       "userInvitations",
		Attributes: attributes,
	}

	if visibleAppIDs != nil {
		data := make([]models.ResourceData, 0, len(visibleAppIDs))
		for _, appID := range visibleAppIDs {
			data = append(data, models.ResourceData{Type: "apps", ID: appID})
		}

		createRequest.Relationships = &models.UserInvitationCreateRelationships{
			VisibleApps: models.ResourceIdentifiers{Data: data},
		}
	}

	requestData := models.Request[models.UserInvitationCreateRequest]{Data: createRequest}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/userInvitations", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.UserInvitation]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteUserInvitation cancels a pending invitation.
//
// The address stops being able to join, but a person who has already accepted
// is a team member and is not affected: removing them is DeleteUser.
func (c *Client) DeleteUserInvitation(invitationID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/userInvitations/%s", c.HostURL, invitationID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
