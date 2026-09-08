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

// GetSubscriptionAvailability reads the territory availability of a
// subscription.
//
// A subscription has exactly one availability record, reachable only through
// the subscription: Apple publishes no collection of them. The record itself
// reports only availableInNewTerritories -- the territory list is a separate
// collection, read by GetSubscriptionAvailableTerritories.
func (c *Client) GetSubscriptionAvailability(subscriptionID string) (*models.SubscriptionAvailability, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/subscriptions/%s/subscriptionAvailability", c.HostURL, subscriptionID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionAvailability]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// A subscription whose availability has never been set answers 200 with a
	// null data member rather than 404. Report that as not found so callers
	// treat it like any other missing resource.
	if response.Data.ID == "" {
		return nil, fmt.Errorf("availability not found for subscription '%s'", subscriptionID)
	}

	return &response.Data, nil
}

// GetSubscriptionAvailableTerritories lists the territories an availability
// record covers.
//
// The list runs to every storefront Apple sells in, so it is paginated like any
// other collection rather than read from the relationship linkage.
func (c *Client) GetSubscriptionAvailableTerritories(availabilityID string) ([]models.Territory, error) {
	return getAllPages[models.Territory](
		c,
		fmt.Sprintf("/v1/subscriptionAvailabilities/%s/availableTerritories", availabilityID),
		defaultPageSize,
	)
}

// CreateSubscriptionAvailability sets the territories a subscription sells in.
//
// This is a replace, not an append. Apple publishes neither PATCH nor DELETE
// for an availability: posting a new one for the same subscription supersedes
// the old one, and there is no way to remove availability altogether -- only to
// narrow the territory list.
//
// A subscription must have an availability before it can be priced. Apple
// rejects POST /v1/subscriptionPrices with a 409 "An error occurred while
// processing the pricing information" until one exists, an error that names
// neither availability nor the territory it is missing.
func (c *Client) CreateSubscriptionAvailability(
	subscriptionID string,
	availableInNewTerritories bool,
	territories []string,
	authToken *string,
) (*models.SubscriptionAvailability, error) {
	territoryRefs := make([]models.ResourceData, 0, len(territories))
	for _, territory := range territories {
		territoryRefs = append(territoryRefs, models.ResourceData{Type: "territories", ID: territory})
	}

	requestData := models.Request[models.SubscriptionAvailabilityCreateRequest]{
		Data: models.SubscriptionAvailabilityCreateRequest{
			Type: "subscriptionAvailabilities",
			Attributes: models.SubscriptionAvailabilityCreateAttributes{
				AvailableInNewTerritories: availableInNewTerritories,
			},
			Relationships: models.SubscriptionAvailabilityCreateRelationships{
				Subscription: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "subscriptions", ID: subscriptionID},
				},
				AvailableTerritories: models.ResourceIdentifiers{Data: territoryRefs},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/subscriptionAvailabilities", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.SubscriptionAvailability]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}
