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

// appInfoIncludes asks Apple to populate the six category linkages.
//
// Without them the relationships come back as links alone, and a category that
// cannot be read cannot be compared against configuration.
const appInfoIncludes = "primaryCategory,primarySubcategoryOne,primarySubcategoryTwo," +
	"secondaryCategory,secondarySubcategoryOne,secondarySubcategoryTwo"

// GetAppInfos retrieves every AppInfo record an app carries.
//
// An app has more than one: the record behind the version on sale, and the
// editable record holding what the next submission will say. Which is which is
// told by the state attribute, not by the order.
func (c *Client) GetAppInfos(appID string) ([]models.AppInfo, error) {
	params := url.Values{}
	params.Set("include", appInfoIncludes)

	return getAllPagesQuery[models.AppInfo](
		c,
		fmt.Sprintf("/v1/apps/%s/appInfos", appID),
		defaultPageSize,
		params,
	)
}

// GetAppInfo retrieves a single AppInfo by its ID.
func (c *Client) GetAppInfo(appInfoID string) (*models.AppInfo, error) {
	params := url.Values{}
	params.Set("include", appInfoIncludes+",app")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/appInfos/%s?%s", c.HostURL, appInfoID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppInfo]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetEditableAppInfo returns the AppInfo an app's next submission will use.
//
// Apple accepts writes only against a record that is still being prepared; one
// in review or already distributed is frozen. This is the AppInfo equivalent of
// EnsureEditableInAppPurchaseVersion, with one important difference: there is
// no POST /v1/appInfos, so when nothing is editable there is nothing to create
// and the caller has to wait for review to finish. The error says so, because
// the raw failure -- a 409 on a PATCH -- does not.
func (c *Client) GetEditableAppInfo(appID string) (*models.AppInfo, error) {
	infos, err := c.GetAppInfos(appID)
	if err != nil {
		return nil, err
	}

	if len(infos) == 0 {
		return nil, fmt.Errorf("no app info records found for app '%s'", appID)
	}

	states := make([]string, 0, len(infos))
	for i := range infos {
		state := ""
		if infos[i].Attributes.State != nil {
			state = string(*infos[i].Attributes.State)
		}
		states = append(states, state)

		for _, editable := range models.EditableAppInfoStates {
			if state == string(editable) {
				return &infos[i], nil
			}
		}
	}

	return nil, fmt.Errorf(
		"app '%s' has no editable app info: every record is in review or already distributed (states: %s). "+
			"App Store Connect metadata can only be written while a version is being prepared, and Apple "+
			"publishes no endpoint to create an app info -- wait for the current review to finish, or start "+
			"a new version",
		appID, strings.Join(states, ", "))
}

// UpdateAppInfoCategories sets an app's App Store categories.
//
// Only the members present in the request are touched: a nil member leaves the
// existing category alone, while a member carrying a null linkage clears it.
// That distinction is the whole reason AppInfoUpdateRelationships uses
// NullableResourceIdentifier rather than the ordinary one.
func (c *Client) UpdateAppInfoCategories(appInfoID string, relationships models.AppInfoUpdateRelationships, authToken *string) (*models.AppInfo, error) {
	requestData := models.Request[models.AppInfoUpdateRequest]{
		Data: models.AppInfoUpdateRequest{
			Type:          "appInfos",
			ID:            appInfoID,
			Relationships: relationships,
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/v1/appInfos/%s", c.HostURL, appInfoID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.AppInfo]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetAppCategories retrieves Apple's category catalogue.
//
// Categories are fixed and account-independent, like territories: the ID is the
// category constant itself -- FINANCE, PRODUCTIVITY, GAMES_PUZZLE -- so a
// category can be assigned without looking one up. This exists so a
// configuration can discover the valid IDs and which platforms accept them.
//
// include=subcategories,parent is requested because a subcategory is only
// meaningful next to the category it belongs to.
func (c *Client) GetAppCategories(platforms []string) ([]models.AppCategory, error) {
	params := url.Values{}
	params.Set("include", "subcategories,parent")
	if len(platforms) > 0 {
		params.Set("filter[platforms]", strings.Join(platforms, ","))
	}

	return getAllPagesQuery[models.AppCategory](c, "/v1/appCategories", defaultPageSize, params)
}
