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

// GetBetaGroupTesters retrieves the testers belonging to one TestFlight group.
func (c *Client) GetBetaGroupTesters(groupID string) ([]models.BetaTester, error) {
	return getAllPages[models.BetaTester](c, fmt.Sprintf("/v1/betaGroups/%s/betaTesters", groupID), defaultPageSize)
}

// GetBetaGroupTesterByEmail finds one tester within a group, or reports that
// the group does not hold them.
//
// The group's own collection is asked rather than the account-wide one because
// the question is membership, not existence: a tester who exists in the account
// and is not in this group must come back as not found, and a collection that
// is already scoped to the group cannot answer that wrongly.
//
// filter[email] narrows the request to a single page where Apple honours it --
// an external group runs to 10,000 testers -- but the match is made again in
// memory, so a filter Apple ignores costs pages rather than correctness.
// Addresses are compared case-insensitively: Apple preserves the case a tester
// was created with and matches without it.
func (c *Client) GetBetaGroupTesterByEmail(groupID, email string) (*models.BetaTester, error) {
	params := url.Values{}
	params.Set("filter[email]", email)

	testers, err := getAllPagesQuery[models.BetaTester](
		c, fmt.Sprintf("/v1/betaGroups/%s/betaTesters", groupID), defaultPageSize, params)
	if err != nil {
		return nil, err
	}

	for i := range testers {
		if attributes := testers[i].Attributes.Email; attributes != nil && strings.EqualFold(*attributes, email) {
			return &testers[i], nil
		}
	}

	return nil, fmt.Errorf("beta tester '%s' not found in beta group '%s'", email, groupID)
}

// GetBetaTesterByEmail finds a tester anywhere in the account.
//
// A beta tester record is the team's rather than one app's -- the same address
// is one record across every app and group -- so this is what decides whether a
// tester has to be created or merely added to a group. A tester who is not in
// the account is reported as not found rather than as an error.
func (c *Client) GetBetaTesterByEmail(email string) (*models.BetaTester, error) {
	params := url.Values{}
	params.Set("filter[email]", email)

	testers, err := getAllPagesQuery[models.BetaTester](c, "/v1/betaTesters", defaultPageSize, params)
	if err != nil {
		return nil, err
	}

	for i := range testers {
		if attributes := testers[i].Attributes.Email; attributes != nil && strings.EqualFold(*attributes, email) {
			return &testers[i], nil
		}
	}

	return nil, fmt.Errorf("beta tester '%s' not found in this App Store Connect account", email)
}

// CreateBetaTester creates a tester record and puts it in a group.
//
// The group relationship is not optional in practice: a tester attached to
// nothing reaches no app, and sending the group here is what makes Apple issue
// the TestFlight invitation. Attributes are accepted only here -- Apple
// publishes no PATCH /v1/betaTesters -- so a tester who already exists keeps
// the name they were created with, and AddBetaTesterToGroup is the call for
// them.
func (c *Client) CreateBetaTester(groupID string, attributes models.BetaTesterCreateAttributes, authToken *string) (*models.BetaTester, error) {
	requestData := models.Request[models.BetaTesterCreateRequest]{
		Data: models.BetaTesterCreateRequest{
			Type:       "betaTesters",
			Attributes: attributes,
			Relationships: models.BetaTesterCreateRelationships{
				BetaGroups: models.ResourceIdentifiers{
					Data: []models.ResourceData{{Type: "betaGroups", ID: groupID}},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/betaTesters", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.BetaTester]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// AddBetaTesterToGroup puts an existing tester in a group.
//
// This is the second half of the create path: an address already in the account
// cannot be created again, so the existing record is linked instead. Apple
// answers 204 with no body.
func (c *Client) AddBetaTesterToGroup(groupID, testerID string, authToken *string) error {
	return c.betaTesterLinkage("POST", groupID, testerID, authToken)
}

// RemoveBetaTesterFromGroup takes a tester out of one group.
//
// This is not DELETE /v1/betaTesters/{id}, which would remove the person from
// every app and group in the account. Membership is what this provider manages,
// so membership is what it withdraws; the record stays in the account's tester
// list.
func (c *Client) RemoveBetaTesterFromGroup(groupID, testerID string, authToken *string) error {
	return c.betaTesterLinkage("DELETE", groupID, testerID, authToken)
}

// betaTesterLinkage sends one tester identifier to a group's membership
// endpoint. POST adds and DELETE removes; both take the same body and answer
// 204 with none.
func (c *Client) betaTesterLinkage(method, groupID, testerID string, authToken *string) error {
	requestData := models.BetaTesterLinkageRequest{
		Data: []models.ResourceData{{Type: "betaTesters", ID: testerID}},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		method,
		fmt.Sprintf("%s/v1/betaGroups/%s/relationships/betaTesters", c.HostURL, groupID),
		strings.NewReader(string(rb)),
	)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
