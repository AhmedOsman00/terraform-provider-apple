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

// TestGetUserScansTheCollection covers the one thing that makes reading a user
// unusual: Apple publishes no GET /v1/users/{id}, so the collection is the only
// read there is and the match has to be made here.
func TestGetUserScansTheCollection(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"type":"users","id":"u1","attributes":{"username":"ada@example.com","roles":["DEVELOPER"]}},
			{"type":"users","id":"u2","attributes":{"username":"grace@example.com","roles":["ADMIN"]}}
		]}`)
	}))
	defer srv.Close()

	user, err := newTestClient(srv).GetUser("u2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/v1/users" {
		t.Errorf("path = %q, want /v1/users — a single-user GET is not published", gotPath)
	}
	if user.Attributes.Username == nil || *user.Attributes.Username != "grace@example.com" {
		t.Errorf("matched the wrong user: %+v", user.Attributes)
	}
}

// TestGetUserReportsAMiss covers the not-found wording the provider's isNotFound
// matches on: a user who has left the team has to read as gone rather than as
// an error.
func TestGetUserReportsAMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetUser("u1")
	if err == nil {
		t.Fatal("expected an error for a user who is not on the team")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to say not found", err.Error())
	}
}

// TestGetUserByUsernameMatchesInMemory covers a filter Apple ignores.
//
// filter[username] is sent, but the match is made again here, so a server that
// answers with the whole team returns the right person rather than the first
// one. The comparison is case-insensitive because Apple preserves the case the
// Apple Account was created with.
func TestGetUserByUsernameMatchesInMemory(t *testing.T) {
	var gotQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("filter[username]")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"type":"users","id":"u1","attributes":{"username":"grace@example.com"}},
			{"type":"users","id":"u2","attributes":{"username":"Ada@Example.com"}}
		]}`)
	}))
	defer srv.Close()

	user, err := newTestClient(srv).GetUserByUsername("ada@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery != "ada@example.com" {
		t.Errorf("filter[username] = %q, want the address", gotQuery)
	}
	if user.ID != "u2" {
		t.Errorf("user ID = %q, want u2 — the in-memory match took the first record instead", user.ID)
	}
}

// TestUpdateUserOmitsVisibleAppsWhenNotRestricting covers the difference
// between "leave app visibility alone" and "make this list exact".
//
// A nil slice must leave the relationship out of the body entirely: sending an
// empty one would take every app away from a member the configuration never
// spoke about.
func TestUpdateUserOmitsVisibleAppsWhenNotRestricting(t *testing.T) {
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"users","id":"u1","attributes":{"roles":["DEVELOPER"]}}}`)
	}))
	defer srv.Close()

	allApps := true
	if _, err := newTestClient(srv).UpdateUser("u1", models.UserUpdateAttributes{
		Roles:          []string{"DEVELOPER"},
		AllAppsVisible: &allApps,
	}, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := gotBody["data"].(map[string]interface{})
	if _, present := data["relationships"]; present {
		t.Error("relationships was sent for a member who is not restricted to a list")
	}
}

// TestUpdateUserSendsAnEmptyVisibleAppsList covers the other half: a member who
// can see no apps is a real answer, and an empty relationship is how it is
// said.
func TestUpdateUserSendsAnEmptyVisibleAppsList(t *testing.T) {
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"users","id":"u1","attributes":{}}}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).UpdateUser("u1", models.UserUpdateAttributes{
		Roles: []string{"DEVELOPER"},
	}, []string{}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(gotBody), `"visibleApps":{"data":[]}`) {
		t.Errorf("body = %s, want an explicit empty visibleApps relationship", gotBody)
	}
}

// TestUpdateUserSendsVisibleApps covers the ordinary restricted case.
func TestUpdateUserSendsVisibleApps(t *testing.T) {
	var gotBody []byte
	var gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"users","id":"u1","attributes":{"allAppsVisible":false}}}`)
	}))
	defer srv.Close()

	restricted := false
	if _, err := newTestClient(srv).UpdateUser("u1", models.UserUpdateAttributes{
		Roles:          []string{"DEVELOPER", "APP_MANAGER"},
		AllAppsVisible: &restricted,
	}, []string{"app1", "app2"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "PATCH" {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}

	var sent models.Request[models.UserUpdateRequest]
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("request body is not the expected shape: %v", err)
	}

	if sent.Data.Type != "users" || sent.Data.ID != "u1" {
		t.Errorf("identity = %q/%q, want users/u1", sent.Data.Type, sent.Data.ID)
	}
	if len(sent.Data.Attributes.Roles) != 2 {
		t.Errorf("roles = %v, want both", sent.Data.Attributes.Roles)
	}
	if sent.Data.Relationships == nil || len(sent.Data.Relationships.VisibleApps.Data) != 2 {
		t.Fatal("visibleApps was not sent")
	}
	if sent.Data.Relationships.VisibleApps.Data[0].Type != "apps" {
		t.Errorf("relationship type = %q, want apps", sent.Data.Relationships.VisibleApps.Data[0].Type)
	}
}

// TestCreateUserInvitationSendsAttributes covers the invitation body.
//
// Apple requires the name here and nowhere else, so a dropped firstName is not
// a cosmetic loss: the request is rejected.
func TestCreateUserInvitationSendsAttributes(t *testing.T) {
	var gotBody []byte
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"userInvitations","id":"i1","attributes":{
			"email":"ada@example.com","roles":["DEVELOPER"],"allAppsVisible":false,
			"expirationDate":"2026-09-25T12:00:00Z"}}}`)
	}))
	defer srv.Close()

	restricted := false
	invitation, err := newTestClient(srv).CreateUserInvitation(models.UserInvitationCreateAttributes{
		Email:          "ada@example.com",
		FirstName:      "Ada",
		LastName:       "Lovelace",
		Roles:          []string{"DEVELOPER"},
		AllAppsVisible: &restricted,
	}, []string{"app1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/v1/userInvitations" {
		t.Errorf("path = %q, want /v1/userInvitations", gotPath)
	}

	var sent models.Request[models.UserInvitationCreateRequest]
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("request body is not the expected shape: %v", err)
	}

	if sent.Data.Attributes.FirstName != "Ada" || sent.Data.Attributes.LastName != "Lovelace" {
		t.Errorf("name = %q %q, want Ada Lovelace — Apple requires both here",
			sent.Data.Attributes.FirstName, sent.Data.Attributes.LastName)
	}
	if len(sent.Data.Attributes.Roles) != 1 || sent.Data.Attributes.Roles[0] != "DEVELOPER" {
		t.Errorf("roles = %v, want [DEVELOPER]", sent.Data.Attributes.Roles)
	}
	if sent.Data.Relationships == nil || len(sent.Data.Relationships.VisibleApps.Data) != 1 {
		t.Fatal("visibleApps was not sent")
	}

	if invitation.ID != "i1" {
		t.Errorf("invitation ID = %q, want i1", invitation.ID)
	}
	if invitation.Attributes.ExpirationDate == nil {
		t.Error("expirationDate was not decoded — it is how a configuration sees the 72-hour window")
	}
}

// TestCreateUserInvitationOmitsVisibleAppsWhenUnrestricted mirrors the user
// case: an invitation granting sight of everything carries no list.
func TestCreateUserInvitationOmitsVisibleAppsWhenUnrestricted(t *testing.T) {
	var gotBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"userInvitations","id":"i1","attributes":{}}}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).CreateUserInvitation(models.UserInvitationCreateAttributes{
		Email:     "ada@example.com",
		FirstName: "Ada",
		LastName:  "Lovelace",
		Roles:     []string{"ADMIN"},
	}, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := gotBody["data"].(map[string]interface{})
	if _, present := data["relationships"]; present {
		t.Error("relationships was sent for an invitation that names no apps")
	}
}

// TestGetUserInvitationByEmailMatchesInMemory covers the same ignored-filter
// hazard GetUserByUsername is guarded against.
func TestGetUserInvitationByEmailMatchesInMemory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
			{"type":"userInvitations","id":"i1","attributes":{"email":"grace@example.com"}},
			{"type":"userInvitations","id":"i2","attributes":{"email":"ada@example.com"}}
		]}`)
	}))
	defer srv.Close()

	invitation, err := newTestClient(srv).GetUserInvitationByEmail("ada@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if invitation.ID != "i2" {
		t.Errorf("invitation ID = %q, want i2", invitation.ID)
	}
}

// TestGetUsersFollowsPagination covers the walk: a team large enough to page is
// rare, and a listing that silently stopped at the first page would drop the
// people at the end of the alphabet.
func TestGetUsersFollowsPagination(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("cursor") == "" {
			fmt.Fprintf(w, `{"data":[{"type":"users","id":"u1","attributes":{"username":"a@example.com"}}],
				"links":{"next":"%s/v1/users?cursor=two"}}`, srv.URL)

			return
		}

		fmt.Fprint(w, `{"data":[{"type":"users","id":"u2","attributes":{"username":"b@example.com"}}]}`)
	}))
	defer srv.Close()

	users, err := newTestClient(srv).GetUsers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("got %d users, want 2 — pagination stopped at the first page", len(users))
	}
}
