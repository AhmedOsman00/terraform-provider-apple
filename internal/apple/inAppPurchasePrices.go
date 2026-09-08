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

// InAppPurchaseManualPrice is one price to schedule.
//
// It is an input helper rather than a wire type: the request Apple wants sends
// each price twice, once as a placeholder in the relationship and once inline
// in "included", and CreateInAppPurchasePriceSchedule assembles both from this.
type InAppPurchaseManualPrice struct {
	PricePointID string
	StartDate    *string
	EndDate      *string
}

// GetInAppPurchasePriceSchedule reads the price schedule of an in-app purchase.
//
// A purchase has exactly one schedule, and it is reachable only through the
// purchase: Apple publishes no collection of schedules. include=baseTerritory
// is requested because the base territory is the one thing on the schedule the
// provider has to compare against configuration, and without the include Apple
// reports it as links alone.
func (c *Client) GetInAppPurchasePriceSchedule(purchaseID string) (*models.InAppPurchasePriceSchedule, error) {
	params := url.Values{}
	params.Set("include", "baseTerritory")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/inAppPurchases/%s/iapPriceSchedule?%s", c.HostURL, purchaseID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchasePriceSchedule]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// A purchase that has never been priced answers 200 with a null data
	// member rather than 404. Report that as not found so callers can treat it
	// the same way they treat a deleted resource.
	if response.Data.ID == "" {
		return nil, fmt.Errorf("price schedule not found for in-app purchase '%s'", purchaseID)
	}

	return &response.Data, nil
}

// GetInAppPurchaseManualPrices reads the prices a developer set explicitly on a
// schedule.
//
// Only manual prices are listed. The prices Apple equalizes into the remaining
// territories from the base territory are a separate, computed collection and
// are never the provider's to manage.
//
// include is required for the result to be usable: without it Apple reports the
// territory and price point relationships as links alone, and a price whose
// price point is unknown cannot be compared against configuration.
func (c *Client) GetInAppPurchaseManualPrices(scheduleID string) ([]models.InAppPurchasePrice, error) {
	params := url.Values{}
	params.Set("include", "territory,inAppPurchasePricePoint")

	return getAllPagesQuery[models.InAppPurchasePrice](
		c,
		fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s/manualPrices", scheduleID),
		defaultPageSize,
		params,
	)
}

// GetInAppPurchasePrices reads an in-app purchase's manual prices in one call.
func (c *Client) GetInAppPurchasePrices(purchaseID string) (*models.InAppPurchasePriceSchedule, []models.InAppPurchasePrice, error) {
	schedule, err := c.GetInAppPurchasePriceSchedule(purchaseID)
	if err != nil {
		return nil, nil, err
	}

	prices, err := c.GetInAppPurchaseManualPrices(schedule.ID)
	if err != nil {
		return nil, nil, err
	}

	return schedule, prices, nil
}

// CreateInAppPurchasePriceSchedule sets the whole price configuration of an
// in-app purchase.
//
// This is a replace, not an append: posting a schedule for a purchase that
// already has one supersedes it, which is why the resource has no update path
// of its own and no delete endpoint exists.
//
// The prices do not exist yet, so they travel inline. Each one is written into
// "included" under a placeholder ID of the form "${price0}", and the
// manualPrices relationship references those same placeholders; Apple
// substitutes real IDs as it commits the write. The placeholder is a literal
// string sent over the wire, not a template the client expands.
func (c *Client) CreateInAppPurchasePriceSchedule(
	purchaseID, baseTerritory string,
	prices []InAppPurchaseManualPrice,
	authToken *string,
) (*models.InAppPurchasePriceSchedule, error) {
	purchaseRef := models.ResourceIdentifier{
		Data: models.ResourceData{Type: "inAppPurchases", ID: purchaseID},
	}

	placeholders := make([]models.ResourceData, 0, len(prices))
	included := make([]models.InAppPurchasePriceInlineCreate, 0, len(prices))

	for i, price := range prices {
		placeholderID := fmt.Sprintf("${price%d}", i)

		placeholders = append(placeholders, models.ResourceData{
			Type: "inAppPurchasePrices",
			ID:   placeholderID,
		})

		included = append(included, models.InAppPurchasePriceInlineCreate{
			Type: "inAppPurchasePrices",
			ID:   placeholderID,
			Attributes: &models.InAppPurchasePriceInlineCreateAttributes{
				StartDate: price.StartDate,
				EndDate:   price.EndDate,
			},
			Relationships: models.InAppPurchasePriceInlineCreateRelationships{
				InAppPurchaseV2: purchaseRef,
				InAppPurchasePricePoint: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "inAppPurchasePricePoints", ID: price.PricePointID},
				},
			},
		})
	}

	requestData := models.InAppPurchasePriceScheduleCreateRequest{
		Data: models.InAppPurchasePriceScheduleCreateData{
			Type: "inAppPurchasePriceSchedules",
			Relationships: models.InAppPurchasePriceScheduleCreateRelationships{
				InAppPurchase: purchaseRef,
				BaseTerritory: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "territories", ID: baseTerritory},
				},
				ManualPrices: models.ResourceIdentifiers{Data: placeholders},
			},
		},
		Included: included,
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/inAppPurchasePriceSchedules", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchasePriceSchedule]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetInAppPurchasePricePoints retrieves the price points available to an in-app
// purchase, optionally narrowed to named territories.
//
// The unfiltered catalogue spans every territory Apple sells in, so the
// territory filter is applied server-side rather than in memory.
// include=territory populates each point's territory linkage, which is what
// makes a returned ID identifiable.
func (c *Client) GetInAppPurchasePricePoints(purchaseID string, territories []string) ([]models.InAppPurchasePricePoint, error) {
	params := url.Values{}
	params.Set("include", "territory")
	if len(territories) > 0 {
		params.Set("filter[territory]", strings.Join(territories, ","))
	}

	return getAllPagesQuery[models.InAppPurchasePricePoint](
		c,
		fmt.Sprintf("/v2/inAppPurchases/%s/pricePoints", purchaseID),
		defaultPageSize,
		params,
	)
}
