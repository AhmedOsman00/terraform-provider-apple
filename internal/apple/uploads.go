// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"bytes"
	"crypto/md5" //nolint:gosec // Apple specifies MD5 for sourceFileChecksum; it is an integrity tag, not a security control.
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// Media uploads run on their own network path.
//
// Every other request this client makes is a small JSON exchange with App Store
// Connect, authenticated with the JWT and bounded by HTTPClient's 30-second
// timeout. An image upload is neither: Apple answers a reservation with
// presigned URLs on its asset storage hosts, and the file parts are PUT there
// as raw bytes. Sending them through doRequest would attach the bearer token to
// a host that is not App Store Connect and force a JSON content type onto a
// binary body, and sharing HTTPClient would hold a multi-megabyte transfer to a
// timeout sized for an API call while it competes for the same connection pool.
// So parts go through UploadHTTPClient -- a separate client with a separate
// transport -- carrying only the headers Apple listed for them.
const uploadTimeout = 5 * time.Minute

// Asset processing is polled briefly after a commit. A screenshot settles in
// seconds, and the poll is what turns "wrong dimensions" into an error at apply
// rather than a red badge in App Store Connect. It is bounded: an asset still
// processing when the poll gives up is returned as it is, not treated as a
// failure. Variables so tests need not wait.
var (
	assetPollInterval = 2 * time.Second
	assetPollAttempts = 30
)

// maxUploadErrorBody caps how much of a failed part's response is quoted back.
const maxUploadErrorBody = 512

func newUploadHTTPClient() *http.Client {
	// A transport of its own, so uploads never share a connection pool with
	// the API client. DefaultTransport is always an *http.Transport unless a
	// program replaces it; fall back to a fresh one if this one has.
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = base.Clone()
	}

	return &http.Client{
		Timeout:   uploadTimeout,
		Transport: transport,
	}
}

var defaultUploadHTTPClient = newUploadHTTPClient()

func (c *Client) uploadClient() *http.Client {
	if c.UploadHTTPClient != nil {
		return c.UploadHTTPClient
	}

	return defaultUploadHTTPClient
}

// MediaChecksum is the checksum Apple records for an uploaded file: the MD5 of
// its bytes, lower-case hex. The provider computes it at plan time from the
// local file and compares it with what Apple reports, so it must be the same
// function on both sides.
func MediaChecksum(data []byte) string {
	sum := md5.Sum(data) //nolint:gosec // see import

	return hex.EncodeToString(sum[:])
}

// mediaKind describes one of Apple's reservation-uploaded asset types.
type mediaKind struct {
	// path is the collection the asset is reserved in, read and deleted from.
	path string
	// resourceType is the JSON:API type of the asset.
	resourceType string
	// parentRelationship and parentType name the relationship the reservation
	// is created under.
	parentRelationship string
	parentType         string
}

var (
	appScreenshotKind = mediaKind{
		path:               "/v1/appScreenshots",
		resourceType:       "appScreenshots",
		parentRelationship: "appScreenshotSet",
		parentType:         "appScreenshotSets",
	}
	subscriptionReviewScreenshotKind = mediaKind{
		path:               "/v1/subscriptionAppStoreReviewScreenshots",
		resourceType:       "subscriptionAppStoreReviewScreenshots",
		parentRelationship: "subscription",
		parentType:         "subscriptions",
	}
	inAppPurchaseReviewScreenshotKind = mediaKind{
		path:               "/v1/inAppPurchaseAppStoreReviewScreenshots",
		resourceType:       "inAppPurchaseAppStoreReviewScreenshots",
		parentRelationship: "inAppPurchaseV2",
		parentType:         "inAppPurchases",
	}
)

// uploadMedia runs Apple's whole upload flow for one file: reserve, deliver
// every part, commit with the checksum, and wait briefly for processing.
//
// A reservation that cannot be completed is deleted rather than left behind:
// an uncommitted asset sits in its parent in AWAITING_UPLOAD, occupying a slot
// -- a screenshot set holds ten -- and showing as a broken image.
func (c *Client) uploadMedia(kind mediaKind, parentID, fileName string, data []byte) (*models.MediaAsset, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("refusing to upload '%s': the file is empty", fileName)
	}

	reserved, err := c.reserveMedia(kind, parentID, fileName, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("could not reserve the upload of '%s': %w", fileName, err)
	}

	abandon := func(cause error) error {
		if delErr := c.deleteMedia(kind, reserved.ID, nil); delErr != nil {
			return fmt.Errorf("%w (the reservation %s could not be removed either: %v)", cause, reserved.ID, delErr)
		}

		return cause
	}

	if err := c.uploadParts(reserved.Attributes.UploadOperations, bytes.NewReader(data), int64(len(data))); err != nil {
		return nil, abandon(fmt.Errorf("could not upload '%s': %w", fileName, err))
	}

	if _, err := c.commitMedia(kind, reserved.ID, MediaChecksum(data)); err != nil {
		return nil, abandon(fmt.Errorf("could not commit the upload of '%s': %w", fileName, err))
	}

	asset, err := c.waitForMedia(kind, reserved.ID)
	if err != nil {
		return nil, fmt.Errorf("could not read back '%s' after uploading it: %w", fileName, err)
	}

	if asset.DeliveryState() == models.AssetDeliveryFailed {
		return nil, abandon(fmt.Errorf("apple rejected '%s' after processing it: %s",
			fileName, describeAssetErrors(asset)))
	}

	return asset, nil
}

func (c *Client) reserveMedia(kind mediaKind, parentID, fileName string, fileSize int64) (*models.MediaAsset, error) {
	requestData := models.Request[models.MediaAssetCreateRequest]{
		Data: models.MediaAssetCreateRequest{
			Type: kind.resourceType,
			Attributes: models.MediaAssetCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: map[string]models.ResourceIdentifier{
				kind.parentRelationship: {
					Data: models.ResourceData{Type: kind.parentType, ID: parentID},
				},
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.HostURL+kind.path, strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.MediaAsset]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Data.ID == "" {
		return nil, fmt.Errorf("no ID was returned for the reservation")
	}

	return &response.Data, nil
}

// uploadParts delivers each byte range to its presigned URL over the upload
// client. Only the headers Apple listed are sent -- never the API token. Each
// part is read from file as it is sent, so a binary of several hundred
// megabytes is never held in memory whole.
func (c *Client) uploadParts(operations []models.UploadOperation, file io.ReaderAt, size int64) error {
	if len(operations) == 0 {
		return fmt.Errorf("no upload operations were returned for the reservation")
	}

	for i, op := range operations {
		if op.Offset < 0 || op.Length <= 0 || op.Offset+op.Length > size {
			return fmt.Errorf("upload operation %d covers bytes %d-%d of a %d-byte file",
				i, op.Offset, op.Offset+op.Length, size)
		}

		target, err := url.Parse(op.URL)
		if err != nil || target.Host == "" {
			return fmt.Errorf("upload operation %d has an invalid URL", i)
		}
		if target.Scheme != "https" {
			return fmt.Errorf("upload operation %d is not HTTPS (scheme %q); refusing to send the file", i, target.Scheme)
		}

		method := op.Method
		if method == "" {
			method = http.MethodPut
		}

		req, err := http.NewRequest(method, op.URL, io.NewSectionReader(file, op.Offset, op.Length))
		if err != nil {
			return err
		}
		req.ContentLength = op.Length
		for _, h := range op.RequestHeaders {
			req.Header.Set(h.Name, h.Value)
		}

		res, err := c.uploadClient().Do(req)
		if err != nil {
			return fmt.Errorf("upload operation %d of %d failed: %w", i+1, len(operations), err)
		}

		resBody, _ := io.ReadAll(io.LimitReader(res.Body, maxUploadErrorBody))
		_ = res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("upload operation %d of %d failed with status %d: %s",
				i+1, len(operations), res.StatusCode, resBody)
		}
	}

	return nil
}

func (c *Client) commitMedia(kind mediaKind, assetID, checksum string) (*models.MediaAsset, error) {
	requestData := models.Request[models.MediaAssetCommitRequest]{
		Data: models.MediaAssetCommitRequest{
			Type: kind.resourceType,
			ID:   assetID,
			Attributes: models.MediaAssetCommitAttributes{
				SourceFileChecksum: checksum,
				Uploaded:           true,
			},
		},
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s%s/%s", c.HostURL, kind.path, assetID), strings.NewReader(string(rb)))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.MediaAsset]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

func (c *Client) getMedia(kind mediaKind, assetID string) (*models.MediaAsset, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s/%s", c.HostURL, kind.path, assetID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[models.MediaAsset]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

func (c *Client) deleteMedia(kind mediaKind, assetID string, authToken *string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s%s/%s", c.HostURL, kind.path, assetID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)
	return err
}

// waitForMedia polls a committed asset until Apple has finished with it or the
// bounded poll runs out, and returns the last state seen either way.
func (c *Client) waitForMedia(kind mediaKind, assetID string) (*models.MediaAsset, error) {
	var asset *models.MediaAsset
	for attempt := 0; attempt < assetPollAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(assetPollInterval)
		}

		var err error
		asset, err = c.getMedia(kind, assetID)
		if err != nil {
			return nil, err
		}

		switch asset.DeliveryState() {
		case models.AssetDeliveryComplete, models.AssetDeliveryFailed:
			return asset, nil
		}
	}

	return asset, nil
}

// getParentMedia reads the single asset a parent holds through a to-one
// relationship endpoint. A parent with none answers 200 with a null data
// member, reported here as not found the way GetAppStoreReviewDetail does.
func (c *Client) getParentMedia(endpoint, parentDescription string) (*models.MediaAsset, error) {
	req, err := http.NewRequest("GET", c.HostURL+endpoint, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		return nil, err
	}

	response := models.Response[*models.MediaAsset]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Data == nil || response.Data.ID == "" {
		return nil, fmt.Errorf("review screenshot not found for %s", parentDescription)
	}

	return response.Data, nil
}

// describeAssetErrors renders Apple's processing errors for a diagnostic.
func describeAssetErrors(asset *models.MediaAsset) string {
	if asset.Attributes.AssetDeliveryState == nil || len(asset.Attributes.AssetDeliveryState.Errors) == 0 {
		return "no reason given"
	}

	reasons := make([]string, 0, len(asset.Attributes.AssetDeliveryState.Errors))
	for _, e := range asset.Attributes.AssetDeliveryState.Errors {
		reasons = append(reasons, fmt.Sprintf("%s: %s", e.Code, e.Description))
	}

	return strings.Join(reasons, "; ")
}
