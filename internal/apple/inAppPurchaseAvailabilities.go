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

// GetInAppPurchaseAvailability reads the territory availability of an in-app
// purchase.
//
// A purchase has exactly one availability record, reachable only through the
// purchase: Apple publishes no collection of them. The record itself reports
// only availableInNewTerritories -- the territory list is a separate collection,
// read by GetInAppPurchaseAvailableTerritories.
func (c *Client) GetInAppPurchaseAvailability(purchaseID string) (*models.InAppPurchaseAvailability, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/inAppPurchases/%s/inAppPurchaseAvailability", c.HostURL, purchaseID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchaseAvailability]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// A purchase whose availability has never been set answers 200 with a null
	// data member rather than 404. Report that as not found so callers treat it
	// like any other missing resource.
	if response.Data.ID == "" {
		return nil, fmt.Errorf("availability not found for in-app purchase '%s'", purchaseID)
	}

	return &response.Data, nil
}

// GetInAppPurchaseAvailableTerritories lists the territories an availability
// record covers.
//
// The list runs to every storefront Apple sells in, so it is paginated like any
// other collection rather than read from the relationship linkage.
func (c *Client) GetInAppPurchaseAvailableTerritories(availabilityID string) ([]models.Territory, error) {
	return getAllPages[models.Territory](
		c,
		fmt.Sprintf("/v1/inAppPurchaseAvailabilities/%s/availableTerritories", availabilityID),
		defaultPageSize,
	)
}

// CreateInAppPurchaseAvailability sets the territories an in-app purchase sells
// in.
//
// This is a replace, not an append. Apple publishes neither PATCH nor DELETE
// for an availability: posting a new one for the same purchase supersedes the
// old one, and there is no way to remove availability altogether -- only to
// narrow the territory list.
func (c *Client) CreateInAppPurchaseAvailability(
	purchaseID string,
	availableInNewTerritories bool,
	territories []string,
	authToken *string,
) (*models.InAppPurchaseAvailability, error) {
	territoryRefs := make([]models.ResourceData, 0, len(territories))
	for _, territory := range territories {
		territoryRefs = append(territoryRefs, models.ResourceData{Type: "territories", ID: territory})
	}

	requestData := models.Request[models.InAppPurchaseAvailabilityCreateRequest]{
		Data: models.InAppPurchaseAvailabilityCreateRequest{
			Type: "inAppPurchaseAvailabilities",
			Attributes: models.InAppPurchaseAvailabilityCreateAttributes{
				AvailableInNewTerritories: availableInNewTerritories,
			},
			Relationships: models.InAppPurchaseAvailabilityCreateRelationships{
				InAppPurchase: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "inAppPurchases", ID: purchaseID},
				},
				AvailableTerritories: models.ResourceIdentifiers{Data: territoryRefs},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/inAppPurchaseAvailabilities", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchaseAvailability]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}
