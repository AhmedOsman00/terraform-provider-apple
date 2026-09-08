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

// GetSubscriptions retrieves every subscription in a subscription group.
//
// As with groups, Apple publishes no top-level collection: a subscription is
// reachable only through its group.
func (c *Client) GetSubscriptions(groupID string) ([]models.Subscription, error) {
	return getAllPages[models.Subscription](c, fmt.Sprintf("/v1/subscriptionGroups/%s/subscriptions", groupID), defaultPageSize)
}

// GetSubscription retrieves a specific subscription by its ID.
//
// include=group is requested so the response populates the group linkage.
// Without it Apple returns the relationship as links alone, and an import would
// have no way to learn which group the subscription belongs to.
func (c *Client) GetSubscription(subscriptionID string) (*models.Subscription, error) {
	params := url.Values{}
	params.Set("include", "group")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/subscriptions/%s?%s", c.HostURL, subscriptionID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Subscription]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateSubscription creates an auto-renewable subscription in a group.
func (c *Client) CreateSubscription(groupID string, attributes models.SubscriptionCreateAttributes, authToken *string) (*models.Subscription, error) {
	requestData := models.Request[models.SubscriptionCreateRequest]{
		Data: models.SubscriptionCreateRequest{
			Type:       "subscriptions",
			Attributes: attributes,
			Relationships: models.SubscriptionCreateRelationships{
				Group: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "subscriptionGroups", ID: groupID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/subscriptions", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Subscription]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateSubscription updates the mutable attributes of a subscription.
//
// productId is not among them: Apple's update request has no such member, so a
// changed product identifier must replace the resource.
func (c *Client) UpdateSubscription(subscriptionID string, attributes models.SubscriptionUpdateAttributes, authToken *string) (*models.Subscription, error) {
	requestData := models.Request[models.SubscriptionUpdateRequest]{
		Data: models.SubscriptionUpdateRequest{
			Type:       "subscriptions",
			ID:         subscriptionID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/subscriptions/%s", c.HostURL, subscriptionID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.Subscription]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteSubscription deletes a subscription.
//
// Apple only permits this while the subscription has never been approved. Once
// it has been available for sale it can be removed from sale but not deleted,
// and the request fails.
func (c *Client) DeleteSubscription(subscriptionID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/subscriptions/%s", c.HostURL, subscriptionID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// GetSubscriptionByProductID finds a subscription by its product identifier
// within one group. Product identifiers are unique across the whole account,
// but Apple offers no endpoint that searches by one.
func (c *Client) GetSubscriptionByProductID(groupID, productID string) (*models.Subscription, error) {
	subscriptions, err := c.GetSubscriptions(groupID)
	if err != nil {
		return nil, err
	}

	for _, subscription := range subscriptions {
		if subscription.Attributes.ProductID == productID {
			return &subscription, nil
		}
	}

	return nil, fmt.Errorf("subscription with product ID '%s' not found in group '%s'", productID, groupID)
}
