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

// AppManualPrice is one price to schedule.
//
// It is an input helper rather than a wire type: the request Apple wants sends
// each price twice, once as a placeholder in the relationship and once inline
// in "included", and CreateAppPriceSchedule assembles both from this.
type AppManualPrice struct {
	PricePointID string
	StartDate    *string
	EndDate      *string
}

// GetAppPriceSchedule reads an app's price schedule.
//
// An app has exactly one, reachable only through the app: Apple publishes no
// collection of schedules. include=baseTerritory is requested because the base
// territory is the one thing on the schedule to compare against configuration,
// and without the include Apple reports it as links alone.
func (c *Client) GetAppPriceSchedule(appID string) (*models.AppPriceSchedule, error) {
	params := url.Values{}
	params.Set("include", "baseTerritory")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/apps/%s/appPriceSchedule?%s", c.HostURL, appID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppPriceSchedule]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// An app that has never been priced answers 200 with a null data member
	// rather than 404. Report that as not found so callers can treat it the
	// same way they treat a deleted resource.
	if response.Data.ID == "" {
		return nil, fmt.Errorf("price schedule not found for app '%s'", appID)
	}

	return &response.Data, nil
}

// GetAppManualPrices reads the prices a developer set explicitly on a schedule.
//
// Only manual prices are listed. The prices Apple equalizes into the remaining
// territories from the base territory are a separate, computed collection and
// are never the provider's to manage.
func (c *Client) GetAppManualPrices(scheduleID string) ([]models.AppPrice, error) {
	params := url.Values{}
	params.Set("include", "territory,appPricePoint")

	return getAllPagesQuery[models.AppPrice](
		c,
		fmt.Sprintf("/v1/appPriceSchedules/%s/manualPrices", scheduleID),
		defaultPageSize,
		params,
	)
}

// GetAppPrices reads an app's schedule and its manual prices in one call.
func (c *Client) GetAppPrices(appID string) (*models.AppPriceSchedule, []models.AppPrice, error) {
	schedule, err := c.GetAppPriceSchedule(appID)
	if err != nil {
		return nil, nil, err
	}

	prices, err := c.GetAppManualPrices(schedule.ID)
	if err != nil {
		return nil, nil, err
	}

	return schedule, prices, nil
}

// CreateAppPriceSchedule sets the whole price configuration of an app.
//
// This is a replace, not an append: posting a schedule for an app that already
// has one supersedes it, which is why the resource has no update path of its
// own and no delete endpoint exists.
//
// The prices do not exist yet, so they travel inline. Each one is written into
// "included" under a placeholder ID of the form "${price0}", and the
// manualPrices relationship references those same placeholders; Apple
// substitutes real IDs as it commits the write. The placeholder is a literal
// string sent over the wire, not a template the client expands.
func (c *Client) CreateAppPriceSchedule(
	appID, baseTerritory string,
	prices []AppManualPrice,
	authToken *string,
) (*models.AppPriceSchedule, error) {
	placeholders := make([]models.ResourceData, 0, len(prices))
	included := make([]models.AppPriceInlineCreate, 0, len(prices))

	for i, price := range prices {
		placeholderID := fmt.Sprintf("${price%d}", i)

		placeholders = append(placeholders, models.ResourceData{
			Type: "appPrices",
			ID:   placeholderID,
		})

		included = append(included, models.AppPriceInlineCreate{
			Type: "appPrices",
			ID:   placeholderID,
			Attributes: &models.AppPriceInlineCreateAttributes{
				StartDate: price.StartDate,
				EndDate:   price.EndDate,
			},
			Relationships: models.AppPriceInlineCreateRelationships{
				AppPricePoint: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "appPricePoints", ID: price.PricePointID},
				},
			},
		})
	}

	requestData := models.AppPriceScheduleCreateRequest{
		Data: models.AppPriceScheduleCreateData{
			Type: "appPriceSchedules",
			Relationships: models.AppPriceScheduleCreateRelationships{
				App: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "apps", ID: appID},
				},
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

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/appPriceSchedules", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppPriceSchedule]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetAppPricePoints retrieves the price points available to an app, optionally
// narrowed to named territories.
//
// The unfiltered catalogue spans every territory Apple sells in, so the
// territory filter is applied server-side rather than in memory.
// include=territory populates each point's territory linkage, which is what
// makes a returned ID identifiable.
func (c *Client) GetAppPricePoints(appID string, territories []string) ([]models.AppPricePoint, error) {
	params := url.Values{}
	params.Set("include", "territory")
	if len(territories) > 0 {
		params.Set("filter[territory]", strings.Join(territories, ","))
	}

	return getAllPagesQuery[models.AppPricePoint](
		c,
		fmt.Sprintf("/v1/apps/%s/appPricePoints", appID),
		defaultPageSize,
		params,
	)
}
