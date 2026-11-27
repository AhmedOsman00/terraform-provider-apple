// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// inAppPurchaseVersionCreateAttempts and inAppPurchaseVersionCreateBackoff
// bound the retry in EnsureEditableInAppPurchaseVersion. The backoff is a
// variable so tests can shorten it.
const inAppPurchaseVersionCreateAttempts = 3

var inAppPurchaseVersionCreateBackoff = 2 * time.Second

// editableVersionStates are the states in which a version's localizations can
// still be changed. Everything else is either in review or already superseded.
var editableVersionStates = map[models.InAppPurchaseVersionState]bool{
	models.InAppPurchaseVersionStatePrepareForSubmission: true,
	models.InAppPurchaseVersionStateDeveloperRejected:    true,
	models.InAppPurchaseVersionStateRejected:             true,
}

// GetInAppPurchaseVersions retrieves the draft versions of an in-app purchase.
func (c *Client) GetInAppPurchaseVersions(purchaseID string) ([]models.InAppPurchaseVersion, error) {
	return getAllPages[models.InAppPurchaseVersion](
		c,
		fmt.Sprintf("/v2/inAppPurchases/%s/versions", purchaseID),
		defaultPageSize,
	)
}

// GetInAppPurchaseVersion retrieves a specific version by its ID.
//
// include=inAppPurchase populates the parent linkage, which is what lets a
// localization be traced back to the purchase it describes: a localization
// names only its version.
func (c *Client) GetInAppPurchaseVersion(versionID string) (*models.InAppPurchaseVersion, error) {
	params := url.Values{}
	params.Set("include", "inAppPurchase")

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/inAppPurchaseVersions/%s?%s", c.HostURL, versionID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchaseVersion]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// CreateInAppPurchaseVersion creates a draft version of an in-app purchase.
//
// A version carries no attributes: it is a snapshot of the purchase's localized
// metadata taken for App Review. Apple publishes no DELETE for one.
func (c *Client) CreateInAppPurchaseVersion(purchaseID string, authToken *string) (*models.InAppPurchaseVersion, error) {
	requestData := models.Request[models.InAppPurchaseVersionCreateRequest]{
		Data: models.InAppPurchaseVersionCreateRequest{
			Type: "inAppPurchaseVersions",
			Relationships: models.InAppPurchaseVersionCreateRelationships{
				InAppPurchase: models.ResourceIdentifier{
					Data: models.ResourceData{Type: "inAppPurchases", ID: purchaseID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/inAppPurchaseVersions", c.HostURL), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, authToken)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.InAppPurchaseVersion]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// GetEditableInAppPurchaseVersion returns the version whose localizations can
// still be edited, or nil when every version is in review or already approved.
//
// The highest version number wins when several qualify, which is the draft App
// Store Connect itself would put an edit into.
func (c *Client) GetEditableInAppPurchaseVersion(purchaseID string) (*models.InAppPurchaseVersion, error) {
	versions, err := c.GetInAppPurchaseVersions(purchaseID)
	if err != nil {
		return nil, err
	}

	var best *models.InAppPurchaseVersion
	for i, version := range versions {
		if version.Attributes.State == nil || !editableVersionStates[*version.Attributes.State] {
			continue
		}
		if best == nil || versionNumber(versions[i]) > versionNumber(*best) {
			best = &versions[i]
		}
	}

	return best, nil
}

// EnsureEditableInAppPurchaseVersion returns a version that accepts metadata
// edits, creating one when none does.
//
// Apple moved in-app purchase localizations behind versions in App Store
// Connect API 4.4.1: a localization is created against a version, never against
// the purchase. A purchase that has never been submitted already has a draft,
// so the create path is only reached for a purchase whose metadata was approved
// and is being changed again -- exactly the point at which App Store Connect's
// own UI opens a new version. Versions cannot be deleted, so the one created
// here outlives the Terraform resource that caused it.
//
// The create is retried on a 5xx, looking for an editable version before each
// retry, the way CreateProfile looks for its profile. Apple answers
// POST /v1/inAppPurchaseVersions with a 500 "An unexpected error occurred" when
// it overlaps the availability and price schedule writes on a new purchase,
// which is how Terraform applies a configuration that creates all three, and
// it creates nothing when it does: the list is still empty afterwards, and the
// next attempt succeeds. Checked against the live API, November 2026.
func (c *Client) EnsureEditableInAppPurchaseVersion(purchaseID string, authToken *string) (*models.InAppPurchaseVersion, error) {
	var lastErr error
	for attempt := 0; attempt < inAppPurchaseVersionCreateAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(inAppPurchaseVersionCreateBackoff)
		}

		// Looking first is what makes a retry safe: a 500 that arrived after
		// Apple created the version leaves one to adopt, and a second create
		// would leave two drafts behind, since versions cannot be deleted.
		version, err := c.GetEditableInAppPurchaseVersion(purchaseID)
		if err != nil {
			return nil, err
		}
		if version != nil {
			return version, nil
		}

		version, err = c.CreateInAppPurchaseVersion(purchaseID, authToken)
		if err == nil {
			return version, nil
		}
		if !isRetryableServerError(err) {
			return nil, err
		}
		lastErr = err
	}

	return nil, fmt.Errorf("creating a version of in-app purchase %q failed after %d attempts: %w",
		purchaseID, inAppPurchaseVersionCreateAttempts, lastErr)
}

// versionNumber reads a version number for ordering, treating an unreported one
// as the oldest.
func versionNumber(version models.InAppPurchaseVersion) int {
	if version.Attributes.Version == nil {
		return 0
	}
	return *version.Attributes.Version
}
