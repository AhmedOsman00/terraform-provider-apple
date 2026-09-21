// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// TestCreateBetaGroupSendsAttributes covers the create request shape.
//
// isInternalGroup and hasAccessToAllBuilds can only be set here -- Apple's
// update request carries neither -- so an attribute dropped from this body is
// not an error, it is a group silently created as the wrong kind.
func TestCreateBetaGroupSendsAttributes(t *testing.T) {
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"betaGroups","id":"g1","attributes":{
			"name":"QA","isInternalGroup":true,"hasAccessToAllBuilds":true,"feedbackEnabled":true},
			"relationships":{"app":{"data":{"type":"apps","id":"app1"}}}}}`)
	}))
	defer srv.Close()

	internal := true
	group, err := newTestClient(srv).CreateBetaGroup("app1", models.BetaGroupCreateAttributes{
		Name:                 "QA",
		IsInternalGroup:      &internal,
		HasAccessToAllBuilds: &internal,
		FeedbackEnabled:      &internal,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sent models.Request[models.BetaGroupCreateRequest]
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("request body is not the expected shape: %v", err)
	}

	if sent.Data.Type != "betaGroups" {
		t.Errorf("type = %q, want betaGroups", sent.Data.Type)
	}
	if sent.Data.Attributes.Name != "QA" {
		t.Errorf("name = %q, want QA", sent.Data.Attributes.Name)
	}
	if sent.Data.Attributes.IsInternalGroup == nil || !*sent.Data.Attributes.IsInternalGroup {
		t.Error("isInternalGroup was not sent")
	}
	if sent.Data.Attributes.HasAccessToAllBuilds == nil || !*sent.Data.Attributes.HasAccessToAllBuilds {
		t.Error("hasAccessToAllBuilds was not sent")
	}
	if sent.Data.Relationships.App.Data.ID != "app1" {
		t.Errorf("app relationship = %q, want app1", sent.Data.Relationships.App.Data.ID)
	}

	if group.ID != "g1" {
		t.Errorf("group ID = %q, want g1", group.ID)
	}
	if group.Relationships == nil || group.Relationships.App == nil || group.Relationships.App.Data.ID != "app1" {
		t.Error("the owning app was not decoded from the response")
	}
}

// TestUpdateBetaGroupOmitsUnsetAttributes covers the pointer-and-omitempty
// shape of the update request: an attribute the configuration does not speak to
// must be absent from the body rather than sent as false, which would turn a
// setting off that nobody asked about.
func TestUpdateBetaGroupOmitsUnsetAttributes(t *testing.T) {
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"betaGroups","id":"g1","attributes":{"name":"Renamed"}}}`)
	}))
	defer srv.Close()

	name := "Renamed"
	if _, err := newTestClient(srv).UpdateBetaGroup("g1", models.BetaGroupUpdateAttributes{Name: &name}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, ok := gotBody["data"].(map[string]interface{})
	if !ok {
		t.Fatal("request body has no data member")
	}

	attributes, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("request body has no attributes member")
	}

	if len(attributes) != 1 {
		t.Errorf("attributes = %v, want name alone", attributes)
	}
	if attributes["name"] != "Renamed" {
		t.Errorf("name = %v, want Renamed", attributes["name"])
	}
}

// TestGetBetaGroupByNameScansTheAppsCollection covers the by-name lookup.
//
// Apple publishes no server-side name filter reachable through the app, so this
// is a full-collection scan -- and it depends on pagination being complete, the
// way every other by-identifier helper in this client does.
func TestGetBetaGroupByNameScansTheAppsCollection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"type":"betaGroups","id":"g1","attributes":{"name":"QA"}},
			{"type":"betaGroups","id":"g2","attributes":{"name":"Public beta"}}]}`)
	}))
	defer srv.Close()

	group, err := newTestClient(srv).GetBetaGroupByName("app1", "Public beta")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.ID != "g2" {
		t.Errorf("group ID = %q, want g2", group.ID)
	}

	if _, err := newTestClient(srv).GetBetaGroupByName("app1", "Nope"); err == nil {
		t.Error("expected an error for a name no group carries")
	}
}

// TestGetBetaAppReviewDetailReportsNullAsNotFound covers Apple's answer for a
// to-one relationship with no value: 200 with a null data member rather than a
// 404. Decoding that into a zero-valued record and handing it back would put an
// empty ID into state.
func TestGetBetaAppReviewDetailReportsNullAsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetBetaAppReviewDetail("app1")
	if err == nil {
		t.Fatal("expected an error when Apple reports no beta app review detail")
	}
	if got := err.Error(); got != "beta app review detail not found for app 'app1'" {
		t.Errorf("error = %q, want the not-found message", got)
	}
}

// TestGetBetaBuildLocalizationByLocaleMatchesExactly pins the locale lookup the
// build-note resource adopts through: a build carries at most one record per
// locale, and matching the wrong one would patch a language nobody asked about.
func TestGetBetaBuildLocalizationByLocaleMatchesExactly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"type":"betaBuildLocalizations","id":"l1","attributes":{"locale":"en-US","whatsNew":"First"}},
			{"type":"betaBuildLocalizations","id":"l2","attributes":{"locale":"en-GB","whatsNew":"Second"}}]}`)
	}))
	defer srv.Close()

	localization, err := newTestClient(srv).GetBetaBuildLocalizationByLocale("b1", "en-GB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if localization.ID != "l2" {
		t.Errorf("localization ID = %q, want l2", localization.ID)
	}

	if _, err := newTestClient(srv).GetBetaBuildLocalizationByLocale("b1", "ar-SA"); err == nil {
		t.Error("expected an error for a locale the build has no note in")
	}
}
