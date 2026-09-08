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

// GetSubscriptionPrices retrieves the price schedule of a subscription.
//
// include is required for the result to be usable: without it Apple returns the
// territory and price point relationships as links alone, and a price whose
// price point is unknown cannot be compared against configuration.
func (c *Client) GetSubscriptionPrices(subscriptionID string) ([]models.SubscriptionPrice, error) {
	params := url.Values{}
	params.Set("include", "territory,subscriptionPricePoint")

	return getAllPagesQuery[models.SubscriptionPrice](
		c,
		fmt.Sprintf("/v1/subscriptions/%s/prices", subscriptionID),
		defaultPageSize,
		params,
	)
}

// GetSubscriptionPrice finds one scheduled price by its ID.
//
// Apple publishes no GET /v1/subscriptionPrices/{id} -- only the collection
// hanging off the subscription -- so this lists that collection and scans it,
// the same shape the bundle ID capability resource is forced into. The parent
// subscription therefore has to be known before a price can be read.
func (c *Client) GetSubscriptionPrice(subscriptionID, priceID string) (*models.SubscriptionPrice, error) {
	prices, err := c.GetSubscriptionPrices(subscriptionID)
	if err != nil {
		return nil, err
	}

	for _, price := range prices {
		if price.ID == priceID {
			return &price, nil
		}
	}

	return nil, fmt.Errorf("price '%s' not found on subscription '%s'", priceID, subscriptionID)
}

// CreateSubscriptionPrice schedules a price for a subscription.
//
// The price itself is never a number: pricePointID references an entry in
// Apple's catalogue, which fixes the customer price and the proceeds for one
// territory. Use GetSubscriptionPricePoints to find one.
func (c *Client) CreateSubscriptionPrice(
	subscriptionID, pricePointID string,
	territoryID *string,
	attributes *models.SubscriptionPriceCreateAttributes,
	authToken *string,
) (*models.SubscriptionPrice, error) {
	relationships := models.SubscriptionPriceCreateRelationships{
		Subscription: models.ResourceIdentifier{
			Data: models.ResourceData{Type: "subscriptions", ID: subscriptionID},
		},
		SubscriptionPricePoint: models.ResourceIdentifier{
			Data: models.ResourceData{Type: "subscriptionPricePoints", ID: pricePointID},
		},
	}

	if territoryID != nil && *territoryID != "" {
		relationships.Territory = &models.ResourceIdentifier{
			Data: models.ResourceData{Type: "territories", ID: *territoryID},
		}
	}

	requestData := models.Request[models.SubscriptionPriceCreateRequest]{
		Data: models.SubscriptionPriceCreateRequest{
			Type:          "subscriptionPrices",
			Attributes:    attributes,
			Relationships: relationships,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/subscriptionPrices", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionPrice]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteSubscriptionPrice removes a scheduled price.
//
// Apple accepts this for a price that has not taken effect yet. Deleting the
// only price in force leaves the subscription without one, which returns it to
// MISSING_METADATA rather than failing.
func (c *Client) DeleteSubscriptionPrice(priceID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/subscriptionPrices/%s", c.HostURL, priceID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// GetSubscriptionPricePoints retrieves the price points available to a
// subscription, optionally narrowed to one territory.
//
// The unfiltered catalogue spans every territory Apple sells in and runs to
// tens of thousands of records, so the territory filter is applied server-side
// rather than in memory. include=territory populates each point's territory
// linkage, which is what makes a returned ID identifiable.
func (c *Client) GetSubscriptionPricePoints(subscriptionID string, territories []string) ([]models.SubscriptionPricePoint, error) {
	params := url.Values{}
	params.Set("include", "territory")
	if len(territories) > 0 {
		params.Set("filter[territory]", strings.Join(territories, ","))
	}

	return getAllPagesQuery[models.SubscriptionPricePoint](
		c,
		fmt.Sprintf("/v1/subscriptions/%s/pricePoints", subscriptionID),
		defaultPageSize,
		params,
	)
}
