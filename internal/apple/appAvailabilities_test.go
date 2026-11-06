// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetAppAvailabilityPatchesExistingStorefronts(t *testing.T) {
	patches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/apps/app/appAvailabilityV2":
			_, _ = w.Write([]byte(`{"data":{"id":"availability","type":"appAvailabilities","attributes":{"availableInNewTerritories":false}}}`))
		case r.Method == "GET" && r.URL.Path == "/v2/appAvailabilities/availability/territoryAvailabilities":
			_, _ = w.Write([]byte(`{"data":[{"id":"us","attributes":{"available":false},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},{"id":"cn","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"CHN"}}}}]}`))
		case r.Method == "PATCH":
			var body struct {
				Data struct {
					ID            string                 `json:"id"`
					Type          string                 `json:"type"`
					Attributes    map[string]interface{} `json:"attributes"`
					Relationships json.RawMessage        `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Data.Type != "territoryAvailabilities" || body.Data.Relationships != nil || len(body.Data.Attributes) != 1 {
				t.Errorf("unexpected PATCH: %+v", body)
			}
			want := body.Data.ID == "us"
			if body.Data.Attributes["available"] != want || r.URL.Path != "/v1/territoryAvailabilities/"+body.Data.ID {
				t.Errorf("wrong storefront patch: %+v", body)
			}
			patches++
			_, _ = w.Write([]byte(`{"data":null}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	c := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}
	got, err := c.SetAppAvailability("app", false, []AppTerritoryAvailability{{Territory: "USA"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "availability" || patches != 2 {
		t.Fatalf("id=%s patches=%d", got.ID, patches)
	}
	patches = 0
	if _, err := c.SetAppAvailability("app", true, nil, nil); err == nil {
		t.Fatal("expected unsupported global setting error")
	}
	if patches != 0 {
		t.Fatal("wrote before validating global setting")
	}
}

// An app whose availability was never set answers 404, not 200 with null data,
// and the first write has to POST rather than fail on it.
func TestSetAppAvailabilityCreatesWhenAppHasNone(t *testing.T) {
	for name, get := range map[string]func(http.ResponseWriter){
		"404": func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appAvailabilities' with id 'app'"}]}`))
		},
		"null data": func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"data":null}`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "GET" && r.URL.Path == "/v1/apps/app/appAvailabilityV2":
					get(w)
				case r.Method == "POST" && r.URL.Path == "/v2/appAvailabilities":
					posts++
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"data":{"id":"app","type":"appAvailabilities","attributes":{"availableInNewTerritories":true}}}`))
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(500)
				}
			}))
			defer srv.Close()
			c := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}
			got, err := c.SetAppAvailability("app", true, []AppTerritoryAvailability{{Territory: "USA"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != "app" || posts != 1 {
				t.Fatalf("id=%s posts=%d", got.ID, posts)
			}
		})
	}
}
