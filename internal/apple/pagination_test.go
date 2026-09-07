// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// newTestClient points a client at a test server, bypassing JWT creation.
func newTestClient(srv *httptest.Server) *Client {
	return &Client{
		HostURL:    srv.URL,
		HTTPClient: srv.Client(),
		Token:      "test-token",
	}
}

// recorder collects the request paths a handler saw, safely across goroutines.
type recorder struct {
	mu    sync.Mutex
	paths []string
}

func (r *recorder) add(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths = append(r.paths, path)
}

func (r *recorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

// TestGetAllPagesFollowsNextLinks verifies a collection spanning several pages
// is returned in full rather than truncated to whatever the first response held.
func TestGetAllPagesFollowsNextLinks(t *testing.T) {
	var srv *httptest.Server
	rec := &recorder{}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/bundleIds", func(w http.ResponseWriter, r *http.Request) {
		rec.add(r.URL.String())

		page := func(ids []string, next string) models.ListResponse[models.BundleID] {
			resp := models.ListResponse[models.BundleID]{}
			for _, id := range ids {
				resp.Data = append(resp.Data, models.BundleID{ID: id, Type: "bundleIds"})
			}
			if next != "" {
				resp.Links = &models.PagedDocumentLinks{Next: srv.URL + next}
			}
			return resp
		}

		var resp models.ListResponse[models.BundleID]
		switch r.URL.Query().Get("cursor") {
		case "":
			resp = page([]string{"1", "2"}, "/v1/bundleIds?limit=200&cursor=B")
		case "B":
			resp = page([]string{"3", "4"}, "/v1/bundleIds?limit=200&cursor=C")
		case "C":
			resp = page([]string{"5"}, "")
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	got, err := newTestClient(srv).GetBundleIDs()
	if err != nil {
		t.Fatalf("GetBundleIDs() returned error: %v", err)
	}

	want := []string{"1", "2", "3", "4", "5"}
	if len(got) != len(want) {
		t.Fatalf("GetBundleIDs() returned %d bundle IDs, want %d spread across 3 pages", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("bundle ID at index %d = %q, want %q", i, got[i].ID, id)
		}
	}

	paths := rec.seen()
	if len(paths) != 3 {
		t.Fatalf("server received %d requests (%v), want 3", len(paths), paths)
	}
	if !strings.Contains(paths[0], "limit=200") {
		t.Errorf("first request was %q, want it to ask for the maximum page size", paths[0])
	}
}

// TestGetAllPagesSinglePage covers the common case of a collection that fits in
// one response and supplies no next link.
func TestGetAllPagesSinglePage(t *testing.T) {
	rec := &recorder{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r.URL.String())
		resp := models.ListResponse[models.BundleID]{
			Data: []models.BundleID{{ID: "1", Type: "bundleIds"}},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	defer srv.Close()

	got, err := newTestClient(srv).GetBundleIDs()
	if err != nil {
		t.Fatalf("GetBundleIDs() returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetBundleIDs() returned %d bundle IDs, want 1", len(got))
	}
	if n := len(rec.seen()); n != 1 {
		t.Errorf("server received %d requests, want 1 when no next link is present", n)
	}
}

// TestGetAllPagesRejectsForeignNextLink ensures the bearer token is never
// replayed to a host the client was not configured for.
func TestGetAllPagesRejectsForeignNextLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.ListResponse[models.BundleID]{
			Data:  []models.BundleID{{ID: "1", Type: "bundleIds"}},
			Links: &models.PagedDocumentLinks{Next: "https://attacker.example/v1/bundleIds"},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetBundleIDs(); err == nil {
		t.Fatal("GetBundleIDs() succeeded, want an error refusing the off-host pagination link")
	} else if !strings.Contains(err.Error(), "refusing to follow") {
		t.Errorf("error = %q, want it to report refusing to follow the link", err)
	}
}

// TestDoRequestDoesNotRetryUnauthorized pins the decision to surface a 401
// instead of re-signing an equivalent token and trying again.
func TestDoRequestDoesNotRetryUnauthorized(t *testing.T) {
	rec := &recorder{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.add(r.URL.String())
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetBundleIDs()
	if err == nil {
		t.Fatal("GetBundleIDs() succeeded, want an authentication error")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want an authentication failure message", err)
	}
	if n := len(rec.seen()); n != 1 {
		t.Errorf("server received %d requests, want exactly 1 because a 401 must not be retried", n)
	}
}

// TestDoRequestSurfacesAppleErrorDetail checks the structured error body Apple
// returns is preferred over the raw status line, since the resource layer
// matches on those messages.
func TestDoRequestSurfacesAppleErrorDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		body := models.ErrorResponse{Errors: []models.APIError{{
			Status: "404",
			Code:   "NOT_FOUND",
			Title:  "The specified resource does not exist",
			Detail: "There is no resource of type 'bundleIds' with id 'missing'",
		}}}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encoding response: %v", err)
		}
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetBundleID("missing")
	if err == nil {
		t.Fatal("GetBundleID() succeeded, want a not-found error")
	}
	// The resource layer keys drift detection off the status appearing here.
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want it to contain the 404 status", err)
	}
	if !strings.Contains(err.Error(), "The specified resource does not exist") {
		t.Errorf("error = %q, want it to carry Apple's error title", err)
	}
}
