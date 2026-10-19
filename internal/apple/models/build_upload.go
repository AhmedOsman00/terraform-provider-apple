// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Build upload states. COMPLETE means Apple has the binary and has handed it
// to processing; the resulting Build then goes through its own
// processingState, which is what a wait for VALID follows.
const (
	BuildUploadStateAwaitingUpload = "AWAITING_UPLOAD"
	BuildUploadStateProcessing     = "PROCESSING"
	BuildUploadStateFailed         = "FAILED"
	BuildUploadStateComplete       = "COMPLETE"
)

// BuildUpload is one upload of a binary: the version, build number and
// platform it claims, and the state of Apple's checks on it. The file itself
// is a separate BuildUploadFile reserved under it.
type BuildUpload struct {
	Type          string                    `json:"type"`
	ID            string                    `json:"id"`
	Attributes    BuildUploadAttributes     `json:"attributes"`
	Relationships *BuildUploadRelationships `json:"relationships,omitempty"`
}

type BuildUploadAttributes struct {
	CFBundleShortVersionString string            `json:"cfBundleShortVersionString,omitempty"`
	CFBundleVersion            string            `json:"cfBundleVersion,omitempty"`
	Platform                   string            `json:"platform,omitempty"`
	CreatedDate                *string           `json:"createdDate,omitempty"`
	UploadedDate               *string           `json:"uploadedDate,omitempty"`
	State                      *BuildUploadState `json:"state,omitempty"`
}

// BuildUploadState carries the validation messages altool used to print:
// errors fail the upload, warnings and infos do not.
type BuildUploadState struct {
	State    string        `json:"state"`
	Errors   []StateDetail `json:"errors,omitempty"`
	Warnings []StateDetail `json:"warnings,omitempty"`
	Infos    []StateDetail `json:"infos,omitempty"`
}

type StateDetail struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type BuildUploadRelationships struct {
	Build *NullableResourceIdentifier `json:"build,omitempty"`
}

// CurrentState is the upload's state, or "" when Apple reported none.
func (u BuildUpload) CurrentState() string {
	if u.Attributes.State == nil {
		return ""
	}
	return u.Attributes.State.State
}

// BuildID is the build the upload produced, once Apple has made one.
func (u BuildUpload) BuildID() string {
	if u.Relationships == nil || u.Relationships.Build == nil || u.Relationships.Build.Data == nil {
		return ""
	}
	return u.Relationships.Build.Data.ID
}

type BuildUploadCreateRequest struct {
	Type          string                        `json:"type"`
	Attributes    BuildUploadCreateAttributes   `json:"attributes"`
	Relationships map[string]ResourceIdentifier `json:"relationships"`
}

type BuildUploadCreateAttributes struct {
	CFBundleShortVersionString string `json:"cfBundleShortVersionString"`
	CFBundleVersion            string `json:"cfBundleVersion"`
	Platform                   string `json:"platform"`
}

// Build upload file asset types and UTIs. An .ipa is a single ASSET; the
// description and SPI asset types exist for what altool sends alongside a
// macOS package.
const (
	BuildUploadAssetTypeAsset = "ASSET"
	BuildUploadUTIIPA         = "com.apple.ipa"
)

// BuildUploadFile is the reservation for one file of an upload, with the
// presigned parts to deliver it in -- the same shape a screenshot reservation
// has.
type BuildUploadFile struct {
	Type       string                    `json:"type"`
	ID         string                    `json:"id"`
	Attributes BuildUploadFileAttributes `json:"attributes"`
}

type BuildUploadFileAttributes struct {
	FileName           string              `json:"fileName,omitempty"`
	FileSize           int64               `json:"fileSize,omitempty"`
	AssetType          string              `json:"assetType,omitempty"`
	UTI                string              `json:"uti,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
}

type BuildUploadFileCreateRequest struct {
	Type          string                          `json:"type"`
	Attributes    BuildUploadFileCreateAttributes `json:"attributes"`
	Relationships map[string]ResourceIdentifier   `json:"relationships"`
}

type BuildUploadFileCreateAttributes struct {
	FileName  string `json:"fileName"`
	FileSize  int64  `json:"fileSize"`
	AssetType string `json:"assetType"`
	UTI       string `json:"uti"`
}

// BuildUploadFileCommitRequest differs from a screenshot's commit: Apple takes
// an object of checksums, each naming its algorithm, rather than one string.
type BuildUploadFileCommitRequest struct {
	Type       string                          `json:"type"`
	ID         string                          `json:"id"`
	Attributes BuildUploadFileCommitAttributes `json:"attributes"`
}

type BuildUploadFileCommitAttributes struct {
	SourceFileChecksums Checksums `json:"sourceFileChecksums"`
	Uploaded            bool      `json:"uploaded"`
}

type Checksums struct {
	File Checksum `json:"file"`
}

type Checksum struct {
	Hash      string `json:"hash"`
	Algorithm string `json:"algorithm"`
}
