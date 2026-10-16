// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Asset delivery states Apple reports on an uploaded image.
//
// A reservation starts in AWAITING_UPLOAD, moves to UPLOAD_COMPLETE once it is
// committed, and settles on COMPLETE or FAILED after Apple has processed the
// file. FAILED carries the reason -- usually the wrong pixel dimensions for the
// display type -- in Errors.
const (
	AssetDeliveryAwaitingUpload = "AWAITING_UPLOAD"
	AssetDeliveryUploadComplete = "UPLOAD_COMPLETE"
	AssetDeliveryComplete       = "COMPLETE"
	AssetDeliveryFailed         = "FAILED"
)

// MediaAsset is any of the image records App Store Connect uploads through
// reservations: an app screenshot, a subscription's review screenshot, an
// in-app purchase's review screenshot. The three are different Apple resources
// with different relationships, but their attributes and their upload flow are
// identical, so they share one wire type.
type MediaAsset struct {
	Type       string               `json:"type"`
	ID         string               `json:"id"`
	Attributes MediaAssetAttributes `json:"attributes"`
	Links      *ResourceLinks       `json:"links,omitempty"`
}

type MediaAssetAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	SourceFileChecksum *string             `json:"sourceFileChecksum,omitempty"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	AssetToken         string              `json:"assetToken,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
}

// DeliveryState returns the asset's delivery state, or "" when Apple reported
// none.
func (a MediaAsset) DeliveryState() string {
	if a.Attributes.AssetDeliveryState == nil {
		return ""
	}

	return a.Attributes.AssetDeliveryState.State
}

type ImageAsset struct {
	TemplateURL string `json:"templateUrl,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

type AppMediaAssetState struct {
	State    string               `json:"state"`
	Errors   []AppMediaStateError `json:"errors,omitempty"`
	Warnings []AppMediaStateError `json:"warnings,omitempty"`
}

type AppMediaStateError struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// UploadOperation is one part of a reserved upload: a byte range of the file
// and the presigned request that delivers it. Apple splits larger files into
// several; each is sent exactly as described, to a host that is not App Store
// Connect.
type UploadOperation struct {
	Method         string       `json:"method"`
	URL            string       `json:"url"`
	Length         int64        `json:"length"`
	Offset         int64        `json:"offset"`
	RequestHeaders []HTTPHeader `json:"requestHeaders,omitempty"`
}

type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MediaAssetCreateRequest reserves an upload. The relationship names the
// parent -- appScreenshotSet, subscription or inAppPurchaseV2 -- and differs
// per asset kind, which is why it is a map rather than a struct.
type MediaAssetCreateRequest struct {
	Type          string                        `json:"type"`
	Attributes    MediaAssetCreateAttributes    `json:"attributes"`
	Relationships map[string]ResourceIdentifier `json:"relationships"`
}

type MediaAssetCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// MediaAssetCommitRequest tells Apple every part has been delivered.
type MediaAssetCommitRequest struct {
	Type       string                     `json:"type"`
	ID         string                     `json:"id"`
	Attributes MediaAssetCommitAttributes `json:"attributes"`
}

type MediaAssetCommitAttributes struct {
	SourceFileChecksum string `json:"sourceFileChecksum"`
	Uploaded           bool   `json:"uploaded"`
}

// AppScreenshotSet groups the screenshots for one display type -- one device
// size -- under one App Store version localization.
type AppScreenshotSet struct {
	Type          string                         `json:"type"`
	ID            string                         `json:"id"`
	Attributes    AppScreenshotSetAttributes     `json:"attributes"`
	Relationships *AppScreenshotSetRelationships `json:"relationships,omitempty"`
}

type AppScreenshotSetAttributes struct {
	ScreenshotDisplayType string `json:"screenshotDisplayType"`
}

type AppScreenshotSetRelationships struct {
	AppStoreVersionLocalization *ResourceIdentifier `json:"appStoreVersionLocalization,omitempty"`
}

type AppScreenshotSetCreateRequest struct {
	Type          string                              `json:"type"`
	Attributes    AppScreenshotSetAttributes          `json:"attributes"`
	Relationships AppScreenshotSetCreateRelationships `json:"relationships"`
}

type AppScreenshotSetCreateRelationships struct {
	AppStoreVersionLocalization ResourceIdentifier `json:"appStoreVersionLocalization"`
}
