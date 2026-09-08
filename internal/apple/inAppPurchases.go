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

// GetInAppPurchases retrieves every in-app purchase belonging to an app.
//
// As with subscriptions, Apple publishes no top-level collection: GET
// /v2/inAppPurchases does not exist, and a purchase is reachable only through
// the app that owns it.
func (c *Client) GetInAppPurchases(appID string) ([]models.InAppPurchase, error) {
	return getAllPages[models.InAppPurchase](c, fmt.Sprintf("/v1/apps/%s/inAppPurchasesV2", appID), defaultPageSize)
}

// GetInAppPurchase retrieves a specific in-app purchase by its ID.
//
// No include is requested because there is nothing useful to include: Apple's
// InAppPurchaseV2 has no app relationship, so the owning app cannot be read
// back at all.
func (c *Client) GetInAppPurchase(purchaseID string) (*models.InAppPurchase, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/inAppPurchases/%s", c.HostURL, purchaseID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchase]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateInAppPurchase creates a one-time purchase on an app.
func (c *Client) CreateInAppPurchase(appID string, attributes models.InAppPurchaseCreateAttributes, authToken *string) (*models.InAppPurchase, error) {
	requestData := models.Request[models.InAppPurchaseCreateRequest]{
		Data: models.InAppPurchaseCreateRequest{
			Type:       "inAppPurchases",
			Attributes: attributes,
			Relationships: models.InAppPurchaseCreateRelationships{
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

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v2/inAppPurchases", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchase]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// UpdateInAppPurchase updates the mutable attributes of an in-app purchase.
//
// productId and inAppPurchaseType are not among them: Apple's update request
// has neither member, so a change to either must replace the resource.
func (c *Client) UpdateInAppPurchase(purchaseID string, attributes models.InAppPurchaseUpdateAttributes, authToken *string) (*models.InAppPurchase, error) {
	requestData := models.Request[models.InAppPurchaseUpdateRequest]{
		Data: models.InAppPurchaseUpdateRequest{
			Type:       "inAppPurchases",
			ID:         purchaseID,
			Attributes: attributes,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v2/inAppPurchases/%s", c.HostURL, purchaseID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchase]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// DeleteInAppPurchase deletes an in-app purchase.
//
// Apple only permits this while the purchase has never been approved. Once it
// has been available for sale it can be removed from sale but not deleted, and
// its product identifier stays reserved either way.
func (c *Client) DeleteInAppPurchase(purchaseID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v2/inAppPurchases/%s", c.HostURL, purchaseID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// GetInAppPurchaseByProductID finds an in-app purchase by its product
// identifier within one app.
//
// Apple's collection does support filter[productId], but the provider reads the
// whole collection and scans it: the lists are small, and the same walk is what
// confirms the purchase belongs to the app named -- which is the only ownership
// check available, since the purchase itself never reports its app.
func (c *Client) GetInAppPurchaseByProductID(appID, productID string) (*models.InAppPurchase, error) {
	purchases, err := c.GetInAppPurchases(appID)
	if err != nil {
		return nil, err
	}

	for _, purchase := range purchases {
		if purchase.Attributes.ProductID == productID {
			return &purchase, nil
		}
	}

	return nil, fmt.Errorf("in-app purchase with product ID '%s' not found in app '%s'", productID, appID)
}

// InAppPurchaseBelongsToApp reports whether an in-app purchase is one of the
// app's own.
//
// This exists because InAppPurchaseV2 has no app relationship: the only way to
// tie a purchase to an app is to list the app's collection and look for it.
func (c *Client) InAppPurchaseBelongsToApp(appID, purchaseID string) (bool, error) {
	purchases, err := c.GetInAppPurchases(appID)
	if err != nil {
		return false, err
	}

	for _, purchase := range purchases {
		if purchase.ID == purchaseID {
			return true, nil
		}
	}

	return false, nil
}
