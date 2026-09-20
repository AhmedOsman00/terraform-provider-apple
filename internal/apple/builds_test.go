// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// TestGetBuildsSendsFilters covers the query parameters the build lookup rests
// on. Apple filters this collection server-side on every coordinate a person
// has -- filter[version] is the build number and
// filter[preReleaseVersion.version] its train -- so a wrong parameter name is
// not an error, it is a full listing silently scanned for nothing.
func TestGetBuildsSendsFilters(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"type":"builds","id":"b1",
			"attributes":{"version":"42","processingState":"VALID"}}]}`)
	}))
	defer srv.Close()

	builds, err := newTestClient(srv).GetBuilds("app1", "IOS", "1.2.0", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"filter[app]":                        "app1",
		"filter[preReleaseVersion.platform]": "IOS",
		"filter[preReleaseVersion.version]":  "1.2.0",
		"filter[version]":                    "42",
	}
	for key, value := range want {
		if got := gotQuery.Get(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}

	if len(builds) != 1 || builds[0].ID != "b1" {
		t.Fatalf("got %d builds, want 1 with ID b1", len(builds))
	}
	if builds[0].Attributes.Version == nil || *builds[0].Attributes.Version != "42" {
		t.Errorf("build number = %v, want 42", builds[0].Attributes.Version)
	}
}

// TestGetBuildsOmitsEmptyFilters covers the listing behind a failed lookup: the
// same function serves "find build 42" and "what did Apple actually receive for
// this train", and the second only works if an empty argument drops its filter
// rather than sending an empty one, which Apple answers with nothing.
func TestGetBuildsOmitsEmptyFilters(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetBuilds("app1", "IOS", "1.2.0", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := gotQuery["filter[version]"]; ok {
		t.Errorf("filter[version] was sent for an empty build number: %q", gotQuery.Get("filter[version]"))
	}
	if got := gotQuery.Get("filter[preReleaseVersion.version]"); got != "1.2.0" {
		t.Errorf("filter[preReleaseVersion.version] = %q, want %q", got, "1.2.0")
	}
}

// TestGetBuildByNumberNotFound covers the empty result. A lookup that finds
// nothing has to be an error rather than a nil build, because the caller is
// about to attach it.
func TestGetBuildByNumberNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetBuildByNumber("app1", "IOS", "1.2.0", "42"); err == nil {
		t.Fatal("expected an error for a build that does not exist")
	}
}

// TestUpdateAppStoreVersionBuildAttaches covers the linkage request. Apple
// answers 204 with no body, so a decode of the response would fail on success.
func TestUpdateAppStoreVersionBuildAttaches(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   models.AppStoreVersionBuildLinkageRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("request body is not a linkage request: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	buildID := "b1"
	if err := newTestClient(srv).UpdateAppStoreVersionBuild("v1", &buildID, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if want := "/v1/appStoreVersions/v1/relationships/build"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotBody.Data == nil {
		t.Fatal("linkage data is null, want the build")
	}
	if gotBody.Data.Type != "builds" || gotBody.Data.ID != "b1" {
		t.Errorf("linkage = %+v, want builds/b1", *gotBody.Data)
	}
}

// TestUpdateAppStoreVersionBuildDetaches covers the other half of the nullable
// relationship. Detaching is an explicit {"data":null}; an omitted member would
// leave the build in place, which is the whole reason the linkage does not use
// the ordinary ResourceIdentifier.
func TestUpdateAppStoreVersionBuildDetaches(t *testing.T) {
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).UpdateAppStoreVersionBuild("v1", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := `{"data":null}`; gotBody != want {
		t.Errorf("body = %s, want %s", gotBody, want)
	}
}

// TestGetAppStoreVersionBuildNoBuild covers a version with nothing attached.
// A to-one relationship with no value answers {"data":null}, which decodes to a
// zero-valued build rather than failing, so the empty ID is the signal.
func TestGetAppStoreVersionBuildNoBuild(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer srv.Close()

	build, err := newTestClient(srv).GetAppStoreVersionBuild("v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if build != nil {
		t.Errorf("got build %+v, want nil", build)
	}
}
