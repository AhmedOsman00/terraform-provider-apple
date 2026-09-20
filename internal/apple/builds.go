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

// GetBuilds retrieves an app's uploaded builds, narrowed by any of the
// coordinates a person actually has.
//
// Unlike most collections this provider reads, Apple filters this one
// server-side on every field that matters -- filter[version] is the build
// number and filter[preReleaseVersion.version] the marketing version of its
// train -- so an exact build is one request rather than a full listing scanned
// in memory. An empty argument omits its filter, which is what makes the same
// function serve both the precise lookup and the "what did Apple actually
// receive" listing behind a failed one.
//
// processingState is deliberately not a parameter. A build still PROCESSING is
// a build that exists, and filtering it away turns a wait into a "no such
// build" -- the caller checks the state it gets back and says so instead.
func (c *Client) GetBuilds(appID, platform, preReleaseVersion, buildNumber string) ([]models.Build, error) {
	params := url.Values{}
	params.Set("filter[app]", appID)

	if platform != "" {
		params.Set("filter[preReleaseVersion.platform]", platform)
	}
	if preReleaseVersion != "" {
		params.Set("filter[preReleaseVersion.version]", preReleaseVersion)
	}
	if buildNumber != "" {
		params.Set("filter[version]", buildNumber)
	}

	return getAllPagesQuery[models.Build](c, "/v1/builds", defaultPageSize, params)
}

// GetBuildByNumber finds the one build of an app identified by its train and
// build number.
//
// The four coordinates together are unique: Apple refuses a second upload
// carrying a build number the same train has already seen.
func (c *Client) GetBuildByNumber(appID, platform, preReleaseVersion, buildNumber string) (*models.Build, error) {
	builds, err := c.GetBuilds(appID, platform, preReleaseVersion, buildNumber)
	if err != nil {
		return nil, err
	}

	if len(builds) == 0 {
		return nil, fmt.Errorf("build '%s' of version '%s' not found for app '%s' on platform '%s'",
			buildNumber, preReleaseVersion, appID, platform)
	}

	return &builds[0], nil
}

// GetAppStoreVersionBuild retrieves the build currently attached to a version,
// or nil when no build is attached.
//
// A to-one relationship with no value answers {"data":null} rather than a 404,
// so an empty ID is the signal, not an error -- but a 404 is tolerated too,
// since a version that has gone away is not an attachment failure.
func (c *Client) GetAppStoreVersionBuild(versionID string) (*models.Build, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/appStoreVersions/%s/build", c.HostURL, versionID), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req, nil)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			return nil, nil
		}

		return nil, err
	}

	response := models.Response[models.Build]{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Data.ID == "" {
		return nil, nil
	}

	return &response.Data, nil
}

// UpdateAppStoreVersionBuild attaches a build to a version, or detaches the one
// attached when buildID is nil.
//
// Apple answers 204 with no body. The build cannot be set when the version is
// created -- AppStoreVersionCreateRequest carries only the app relationship --
// so this is a second call however the resource is modelled.
func (c *Client) UpdateAppStoreVersionBuild(versionID string, buildID *string, authToken *string) error {
	requestData := models.AppStoreVersionBuildLinkageRequest{}
	if buildID != nil {
		requestData.Data = &models.ResourceData{Type: "builds", ID: *buildID}
	}

	rb, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		"PATCH",
		fmt.Sprintf("%s/v1/appStoreVersions/%s/relationships/build", c.HostURL, versionID),
		strings.NewReader(string(rb)),
	)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req, authToken)

	return err
}
