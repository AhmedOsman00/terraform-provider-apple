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
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// TestCreateInAppPurchasePriceScheduleSendsInlinePrices covers the one request
// in this provider that uses JSON:API's "included" member.
//
// The prices do not exist when the schedule is created, so each one is sent
// inline under a placeholder ID that the manualPrices relationship references.
// If the two ever stop matching, Apple rejects the request -- or worse, commits
// a schedule with no prices in it.
func TestCreateInAppPurchasePriceScheduleSendsInlinePrices(t *testing.T) {
	var got models.InAppPurchasePriceScheduleCreateRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("request body did not decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched1"}}`)
	}))
	defer srv.Close()

	start := "2026-01-01"
	schedule, err := newTestClient(srv).CreateInAppPurchasePriceSchedule(
		"iap1",
		"USA",
		[]InAppPurchaseManualPrice{
			{PricePointID: "point-usa"},
			{PricePointID: "point-gbr", StartDate: &start},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schedule.ID != "sched1" {
		t.Errorf("schedule ID = %q, want %q", schedule.ID, "sched1")
	}

	if got.Data.Type != "inAppPurchasePriceSchedules" {
		t.Errorf("data type = %q, want %q", got.Data.Type, "inAppPurchasePriceSchedules")
	}
	if got.Data.Relationships.BaseTerritory.Data.ID != "USA" {
		t.Errorf("base territory = %q, want %q", got.Data.Relationships.BaseTerritory.Data.ID, "USA")
	}
	if got.Data.Relationships.InAppPurchase.Data.ID != "iap1" {
		t.Errorf("in-app purchase = %q, want %q", got.Data.Relationships.InAppPurchase.Data.ID, "iap1")
	}

	if len(got.Included) != 2 {
		t.Fatalf("got %d included prices, want 2", len(got.Included))
	}
	if len(got.Data.Relationships.ManualPrices.Data) != 2 {
		t.Fatalf("got %d manual price references, want 2", len(got.Data.Relationships.ManualPrices.Data))
	}

	// Every placeholder in the relationship must name an included object, and
	// every included object must carry the purchase and the price point.
	included := make(map[string]models.InAppPurchasePriceInlineCreate, len(got.Included))
	for _, price := range got.Included {
		included[price.ID] = price
	}
	for _, ref := range got.Data.Relationships.ManualPrices.Data {
		if ref.Type != "inAppPurchasePrices" {
			t.Errorf("manual price reference type = %q, want %q", ref.Type, "inAppPurchasePrices")
		}
		price, ok := included[ref.ID]
		if !ok {
			t.Fatalf("manual price %q has no matching included object", ref.ID)
		}
		if price.Relationships.InAppPurchaseV2.Data.ID != "iap1" {
			t.Errorf("included price %q names purchase %q, want %q",
				ref.ID, price.Relationships.InAppPurchaseV2.Data.ID, "iap1")
		}
		if price.Relationships.InAppPurchasePricePoint.Data.ID == "" {
			t.Errorf("included price %q names no price point", ref.ID)
		}
	}

	// The start date travels as a plain date, and a price without one sends
	// null rather than being omitted: Apple reads null as "in effect now".
	for _, price := range got.Included {
		if price.Attributes == nil {
			t.Fatalf("included price %q sent no attributes", price.ID)
		}
		if price.Relationships.InAppPurchasePricePoint.Data.ID == "point-gbr" {
			if price.Attributes.StartDate == nil || *price.Attributes.StartDate != start {
				t.Errorf("start date = %v, want %q", price.Attributes.StartDate, start)
			}
		}
	}
}

// TestGetInAppPurchasePriceScheduleReportsUnpricedAsNotFound covers Apple
// answering 200 with a null data member for a purchase that has never been
// priced. Reported as an error containing "not found" so the resource treats it
// the same way it treats a deleted one.
func TestGetInAppPurchasePriceScheduleReportsUnpricedAsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetInAppPurchasePriceSchedule("iap1")
	if err == nil {
		t.Fatal("expected an error for a purchase with no price schedule, got none")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not found")
	}
}

// TestGetInAppPurchaseAvailabilityReportsMissingAsNotFound is the availability
// counterpart of the price schedule case above.
func TestGetInAppPurchaseAvailabilityReportsMissingAsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).GetInAppPurchaseAvailability("iap1")
	if err == nil {
		t.Fatal("expected an error for a purchase with no availability, got none")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not found")
	}
}

// TestEnsureEditableInAppPurchaseVersion covers version resolution, which is
// what stands between a localization and the purchase it describes.
func TestEnsureEditableInAppPurchaseVersion(t *testing.T) {
	t.Run("picks the highest editable version and creates nothing", func(t *testing.T) {
		created := false

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				created = true
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[
				{"type":"inAppPurchaseVersions","id":"v1","attributes":{"version":1,"state":"APPROVED"}},
				{"type":"inAppPurchaseVersions","id":"v2","attributes":{"version":2,"state":"PREPARE_FOR_SUBMISSION"}},
				{"type":"inAppPurchaseVersions","id":"v3","attributes":{"version":3,"state":"IN_REVIEW"}}
			]}`)
		}))
		defer srv.Close()

		version, err := newTestClient(srv).EnsureEditableInAppPurchaseVersion("iap1", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version.ID != "v2" {
			t.Errorf("version = %q, want %q -- the editable one, not the newest", version.ID, "v2")
		}
		if created {
			t.Error("a version was created even though an editable one existed")
		}
	})

	t.Run("creates a version when every existing one is in review", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{"data":{"type":"inAppPurchaseVersions","id":"v9",
					"attributes":{"version":9,"state":"PREPARE_FOR_SUBMISSION"}}}`)
				return
			}
			fmt.Fprint(w, `{"data":[
				{"type":"inAppPurchaseVersions","id":"v1","attributes":{"version":1,"state":"IN_REVIEW"}}
			]}`)
		}))
		defer srv.Close()

		version, err := newTestClient(srv).EnsureEditableInAppPurchaseVersion("iap1", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version.ID != "v9" {
			t.Errorf("version = %q, want the newly created %q", version.ID, "v9")
		}
	})

	t.Run("a version Apple reported without a state is not editable", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{"data":{"type":"inAppPurchaseVersions","id":"vnew"}}`)
				return
			}
			fmt.Fprint(w, `{"data":[{"type":"inAppPurchaseVersions","id":"v1","attributes":{"version":1}}]}`)
		}))
		defer srv.Close()

		version, err := newTestClient(srv).EnsureEditableInAppPurchaseVersion("iap1", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version.ID != "vnew" {
			t.Errorf("version = %q, want a freshly created one", version.ID)
		}
	})
}

// TestGetInAppPurchasePricePointsSendsFilters covers the query parameters the
// price point catalogue depends on: without include=territory a returned point
// cannot be identified, and without filter[territory] the request pulls every
// territory Apple sells in.
func TestGetInAppPurchasePricePointsSendsFilters(t *testing.T) {
	var gotQuery url.Values
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"type":"inAppPurchasePricePoints","id":"p1",
			"attributes":{"customerPrice":"9.99","proceeds":"6.99"},
			"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}]}`)
	}))
	defer srv.Close()

	points, err := newTestClient(srv).GetInAppPurchasePricePoints("iap1", []string{"USA", "GBR"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/v2/inAppPurchases/iap1/pricePoints" {
		t.Errorf("path = %q, want %q", gotPath, "/v2/inAppPurchases/iap1/pricePoints")
	}
	if got := gotQuery.Get("include"); got != "territory" {
		t.Errorf("include = %q, want %q", got, "territory")
	}
	if got := gotQuery.Get("filter[territory]"); got != "USA,GBR" {
		t.Errorf("filter[territory] = %q, want %q", got, "USA,GBR")
	}

	if len(points) != 1 {
		t.Fatalf("got %d price points, want 1", len(points))
	}
	if points[0].Relationships == nil || points[0].Relationships.Territory == nil {
		t.Fatal("territory relationship was not decoded")
	}
}

// TestCreateInAppPurchaseAvailabilitySendsTerritories covers the to-many
// relationship shape: JSON:API wraps it as one object with an array under
// "data", and Apple rejects a bare array.
func TestCreateInAppPurchaseAvailabilitySendsTerritories(t *testing.T) {
	var got models.Request[models.InAppPurchaseAvailabilityCreateRequest]

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("request body did not decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"data":{"type":"inAppPurchaseAvailabilities","id":"avail1",
			"attributes":{"availableInNewTerritories":false}}}`)
	}))
	defer srv.Close()

	availability, err := newTestClient(srv).CreateInAppPurchaseAvailability("iap1", false, []string{"USA", "EGY"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if availability.ID != "avail1" {
		t.Errorf("availability ID = %q, want %q", availability.ID, "avail1")
	}

	if got.Data.Attributes.AvailableInNewTerritories {
		t.Error("availableInNewTerritories was sent as true, want false")
	}
	territories := got.Data.Relationships.AvailableTerritories.Data
	if len(territories) != 2 {
		t.Fatalf("got %d territories, want 2", len(territories))
	}
	for _, territory := range territories {
		if territory.Type != "territories" {
			t.Errorf("territory type = %q, want %q", territory.Type, "territories")
		}
	}
}
