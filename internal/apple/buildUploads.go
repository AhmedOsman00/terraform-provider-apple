// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"crypto/md5" //nolint:gosec // Apple names MD5 among its checksum algorithms; it is an integrity tag, not a security control.
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// UploadBuild sends an .ipa to App Store Connect the way altool and Transporter
// do, through the build upload API (App Store Connect API 4.1): create a
// buildUpload naming the version, build number and platform, reserve a
// buildUploadFile under it, deliver every part over the upload network, and
// commit with the file's checksum.
//
// It returns once Apple has the file. Apple's checks run after that, so the
// caller polls GetBuildUpload for COMPLETE or FAILED. An upload that cannot be
// delivered or committed is deleted rather than left AWAITING_UPLOAD, which
// would otherwise hold the build number.
func (c *Client) UploadBuild(appID, platform, version, buildNumber, file string) (*models.BuildUpload, error) {
	handle, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	info, err := handle.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("refusing to upload '%s': the file is empty", file)
	}

	checksum := md5.New() //nolint:gosec // see import
	if _, err := io.Copy(checksum, handle); err != nil {
		return nil, fmt.Errorf("could not read '%s': %w", file, err)
	}

	upload, err := c.createBuildUpload(appID, platform, version, buildNumber)
	if err != nil {
		return nil, fmt.Errorf("could not start the upload of %s (%s): %w", version, buildNumber, err)
	}

	abandon := func(cause error) error {
		if delErr := c.DeleteBuildUpload(upload.ID); delErr != nil {
			return fmt.Errorf("%w (the upload %s could not be removed either: %v)", cause, upload.ID, delErr)
		}
		return cause
	}

	name := filepath.Base(file)
	reserved, err := c.reserveBuildUploadFile(upload.ID, name, info.Size())
	if err != nil {
		return nil, abandon(fmt.Errorf("could not reserve the upload of '%s': %w", name, err))
	}

	if err := c.uploadParts(reserved.Attributes.UploadOperations, handle, info.Size()); err != nil {
		return nil, abandon(fmt.Errorf("could not upload '%s': %w", name, err))
	}

	if err := c.commitBuildUploadFile(reserved.ID, hex.EncodeToString(checksum.Sum(nil))); err != nil {
		return nil, abandon(fmt.Errorf("could not commit the upload of '%s': %w", name, err))
	}

	return upload, nil
}

func (c *Client) createBuildUpload(appID, platform, version, buildNumber string) (*models.BuildUpload, error) {
	requestData := models.Request[models.BuildUploadCreateRequest]{
		Data: models.BuildUploadCreateRequest{
			Type: "buildUploads",
			Attributes: models.BuildUploadCreateAttributes{
				CFBundleShortVersionString: version,
				CFBundleVersion:            buildNumber,
				Platform:                   platform,
			},
			Relationships: map[string]models.ResourceIdentifier{
				"app": {Data: models.ResourceData{Type: "apps", ID: appID}},
			},
		},
	}

	response := models.Response[models.BuildUpload]{}
	if err := c.sendJSON("POST", "/v1/buildUploads", requestData, &response); err != nil {
		return nil, err
	}
	if response.Data.ID == "" {
		return nil, fmt.Errorf("no ID was returned for the build upload")
	}

	return &response.Data, nil
}

func (c *Client) reserveBuildUploadFile(uploadID, fileName string, fileSize int64) (*models.BuildUploadFile, error) {
	requestData := models.Request[models.BuildUploadFileCreateRequest]{
		Data: models.BuildUploadFileCreateRequest{
			Type: "buildUploadFiles",
			Attributes: models.BuildUploadFileCreateAttributes{
				FileName:  fileName,
				FileSize:  fileSize,
				AssetType: models.BuildUploadAssetTypeAsset,
				UTI:       models.BuildUploadUTIIPA,
			},
			Relationships: map[string]models.ResourceIdentifier{
				"buildUpload": {Data: models.ResourceData{Type: "buildUploads", ID: uploadID}},
			},
		},
	}

	response := models.Response[models.BuildUploadFile]{}
	if err := c.sendJSON("POST", "/v1/buildUploadFiles", requestData, &response); err != nil {
		return nil, err
	}
	if response.Data.ID == "" {
		return nil, fmt.Errorf("no ID was returned for the reservation")
	}

	return &response.Data, nil
}

func (c *Client) commitBuildUploadFile(fileID, md5Hex string) error {
	requestData := models.Request[models.BuildUploadFileCommitRequest]{
		Data: models.BuildUploadFileCommitRequest{
			Type: "buildUploadFiles",
			ID:   fileID,
			Attributes: models.BuildUploadFileCommitAttributes{
				SourceFileChecksums: models.Checksums{File: models.Checksum{Hash: md5Hex, Algorithm: "MD5"}},
				Uploaded:            true,
			},
		},
	}

	return c.sendJSON("PATCH", "/v1/buildUploadFiles/"+fileID, requestData, nil)
}

// GetBuildUpload reads an upload's state, with the build it produced once
// there is one.
func (c *Client) GetBuildUpload(uploadID string) (*models.BuildUpload, error) {
	response := models.Response[models.BuildUpload]{}
	if err := c.sendJSON("GET", "/v1/buildUploads/"+uploadID, nil, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

func (c *Client) DeleteBuildUpload(uploadID string) error {
	return c.sendJSON("DELETE", "/v1/buildUploads/"+uploadID, nil, nil)
}

// sendJSON is one App Store Connect request: body marshalled when there is one,
// response decoded into out when out is not nil.
func (c *Client) sendJSON(method, endpoint string, body, out any) error {
	var reader io.Reader
	if body != nil {
		rb, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = strings.NewReader(string(rb))
	}

	req, err := http.NewRequest(method, c.HostURL+endpoint, reader)
	if err != nil {
		return err
	}

	responseBody, err := c.doRequest(req, nil)
	if err != nil || out == nil {
		return err
	}

	return json.Unmarshal(responseBody, out)
}

// DescribeStateDetails renders Apple's validation messages for a diagnostic.
func DescribeStateDetails(details []models.StateDetail) string {
	lines := make([]string, 0, len(details))
	for _, detail := range details {
		lines = append(lines, fmt.Sprintf("%s: %s", detail.Code, detail.Description))
	}
	return strings.Join(lines, "; ")
}
