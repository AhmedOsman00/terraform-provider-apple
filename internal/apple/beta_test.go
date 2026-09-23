// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestCreateBetaTesterSendsGroupRelationship covers the create request shape.
//
// The group relationship is what makes a tester reach an app and what makes
// Apple send the invitation; a body that lost it would create a tester attached
// to nothing and report success.
func TestCreateBetaTesterSendsGroupRelationship(t *testing.T) {
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"betaTesters","id":"t1","attributes":{
			"email":"tester@example.com","firstName":"Ada","inviteType":"EMAIL"}}}`)
	}))
	defer srv.Close()

	first := "Ada"
	tester, err := newTestClient(srv).CreateBetaTester("g1", models.BetaTesterCreateAttributes{
		Email:     "tester@example.com",
		FirstName: &first,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sent models.Request[models.BetaTesterCreateRequest]
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("request body is not the expected shape: %v", err)
	}

	if sent.Data.Type != "betaTesters" {
		t.Errorf("type = %q, want betaTesters", sent.Data.Type)
	}
	if sent.Data.Attributes.Email != "tester@example.com" {
		t.Errorf("email = %q, want tester@example.com", sent.Data.Attributes.Email)
	}
	if sent.Data.Attributes.LastName != nil {
		t.Errorf("lastName = %v, want it omitted", *sent.Data.Attributes.LastName)
	}
	if groups := sent.Data.Relationships.BetaGroups.Data; len(groups) != 1 ||
		groups[0].Type != "betaGroups" || groups[0].ID != "g1" {
		t.Errorf("betaGroups relationship = %+v, want the one group g1", groups)
	}

	if tester.ID != "t1" {
		t.Errorf("tester ID = %q, want t1", tester.ID)
	}
	if tester.Attributes.InviteType == nil || *tester.Attributes.InviteType != "EMAIL" {
		t.Error("inviteType was not decoded from the response")
	}
}

// TestBetaTesterLinkageUsesTheGroupRelationship covers adding and removing a
// membership.
//
// Both go to the group's relationships endpoint with the same body and differ
// only by method: a DELETE sent to /v1/betaTesters/{id} instead would remove the
// person from every app in the account.
func TestBetaTesterLinkageUsesTheGroupRelationship(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		call   func(*Client) error
		method string
	}{
		{
			name:   "add",
			call:   func(c *Client) error { return c.AddBetaTesterToGroup("g1", "t1", nil) },
			method: "POST",
		},
		{
			name:   "remove",
			call:   func(c *Client) error { return c.RemoveBetaTesterFromGroup("g1", "t1", nil) },
			method: "DELETE",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var (
				gotMethod string
				gotPath   string
				gotBody   []byte
			)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				gotBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			if err := testCase.call(newTestClient(srv)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotMethod != testCase.method {
				t.Errorf("method = %q, want %q", gotMethod, testCase.method)
			}
			if want := "/v1/betaGroups/g1/relationships/betaTesters"; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}

			var sent models.BetaTesterLinkageRequest
			if err := json.Unmarshal(gotBody, &sent); err != nil {
				t.Fatalf("request body is not the expected shape: %v", err)
			}
			if len(sent.Data) != 1 || sent.Data[0].Type != "betaTesters" || sent.Data[0].ID != "t1" {
				t.Errorf("body data = %+v, want the one tester t1", sent.Data)
			}
		})
	}
}

// TestGetBetaGroupTesterByEmailAsksTheGroupsCollection pins the membership
// lookup.
//
// The question is membership rather than existence, so the collection asked has
// to be the group's own -- a tester who exists in the account and is not in this
// group must come back as not found. No filter is sent: Apple refuses
// filter[email] on the relationship endpoint with a 400, so the collection is
// walked and the match made in memory.
func TestGetBetaGroupTesterByEmailAsksTheGroupsCollection(t *testing.T) {
	var (
		gotPath  string
		gotQuery string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query().Get("filter[email]")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"type":"betaTesters","id":"t1","attributes":{"email":"someone@example.com"}},
			{"type":"betaTesters","id":"t2","attributes":{"email":"Tester@Example.com"}}]}`)
	}))
	defer srv.Close()

	// Apple preserves the case an address was created with and matches without
	// it, so the scan does too.
	tester, err := newTestClient(srv).GetBetaGroupTesterByEmail("g1", "tester@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tester.ID != "t2" {
		t.Errorf("tester ID = %q, want t2", tester.ID)
	}

	if want := "/v1/betaGroups/g1/betaTesters"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotQuery != "" {
		t.Errorf("filter[email] = %q, want it not to be sent: Apple rejects it here", gotQuery)
	}

	_, err = newTestClient(srv).GetBetaGroupTesterByEmail("g1", "nobody@example.com")
	if err == nil {
		t.Fatal("expected an error for an address the group does not hold")
	}
	// The resource reads this as "removed from the group" rather than as a
	// failure, which it recognises by the words.
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to report not found", err.Error())
	}
}

// TestGetBetaTesterByEmailReportsAMissAsNotFound pins the account-wide lookup
// that decides whether a tester is created or merely added to a group.
func TestGetBetaTesterByEmailReportsAMissAsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/betaTesters" {
			t.Errorf("path = %q, want /v1/betaTesters", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetBetaTesterByEmail("tester@example.com")
	if err == nil {
		t.Fatal("expected an error for an address the account does not hold")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to report not found", err.Error())
	}
}
