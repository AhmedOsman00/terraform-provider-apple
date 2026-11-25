// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// priceServer is an App Store Connect that holds one subscription in a given
// state with the given prices, and records every request it is sent.
type priceServer struct {
	mu       sync.Mutex
	requests []string
	bodies   map[string][]string
}

func newPriceServer(t *testing.T, state string, prices string) (*priceServer, *apple.Client) {
	t.Helper()
	ps := &priceServer{bodies: map[string][]string{}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		key := r.Method + " " + r.URL.Path
		ps.mu.Lock()
		ps.requests = append(ps.requests, key)
		ps.bodies[key] = append(ps.bodies[key], string(body))
		ps.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch key {
		case "GET /v1/subscriptions/sub-1":
			fmt.Fprintf(w, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"state":%q}}}`, state)
		case "GET /v1/subscriptions/sub-1/prices":
			fmt.Fprintf(w, `{"data":[%s],"links":{}}`, prices)
		case "PATCH /v1/subscriptions/sub-1":
			fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{}}}`)
		case "POST /v1/subscriptionPrices":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"data":{"type":"subscriptionPrices","id":"price-new","attributes":{}}}`)
		default:
			if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/subscriptionPrices/") {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"errors":[{"status":"404","title":"not found"}]}`)
		}
	}))
	t.Cleanup(srv.Close)

	return ps, &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}
}

func (ps *priceServer) sent(key string) []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.bodies[key]
}

// apiPrice renders one price as Apple lists it.
func apiPrice(id, point, territory, startDate string) string {
	start := "null"
	if startDate != "" {
		start = fmt.Sprintf("%q", startDate)
	}
	return fmt.Sprintf(`{"type":"subscriptionPrices","id":%q,"attributes":{"startDate":%s},`+
		`"relationships":{"territory":{"data":{"type":"territories","id":%q}},`+
		`"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":%q}}}}`,
		id, start, territory, point)
}

func scheduleWith(prices ...subscriptionSchedulePriceModel) *subscriptionPriceScheduleModel {
	return &subscriptionPriceScheduleModel{
		ID:             types.StringUnknown(),
		SubscriptionID: types.StringValue("sub-1"),
		Prices:         prices,
	}
}

func fixClock(t *testing.T) {
	t.Helper()
	previous := now
	now = func() time.Time { return time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = previous })
}

// TestPriceScheduleWriteBeforeApprovalPatchesTheSet checks the initial price
// path is unchanged: one replace-all PATCH and no per-territory POST.
func TestPriceScheduleWriteBeforeApprovalPatchesTheSet(t *testing.T) {
	ps, client := newPriceServer(t, "READY_TO_SUBMIT", "")
	plan := scheduleWith(plannedPrice("usa-8", "USA"))

	var diags diag.Diagnostics
	(&subscriptionPriceScheduleResource{client: client}).write(context.Background(), plan, &diags)

	if diags.HasError() {
		t.Fatalf("write: %v", diags)
	}
	if got := len(ps.sent("PATCH /v1/subscriptions/sub-1")); got != 1 {
		t.Errorf("sent %d PATCH, want 1", got)
	}
	if got := len(ps.sent("POST /v1/subscriptionPrices")); got != 0 {
		t.Errorf("sent %d POST, want none before approval", got)
	}
	if plan.ID.ValueString() != "sub-1" {
		t.Errorf("id = %v, want sub-1", plan.ID)
	}
}

// TestPriceScheduleWriteAfterApprovalPostsADatedChange is the case the PATCH
// cannot do: Apple refuses an initial price on an approved subscription, so a
// change has to be a dated POST in its territory, and only there.
func TestPriceScheduleWriteAfterApprovalPostsADatedChange(t *testing.T) {
	fixClock(t)
	ps, client := newPriceServer(t, "APPROVED", strings.Join([]string{
		apiPrice("p-usa", "usa-8", "USA", ""),
		apiPrice("p-gbr", "gbr-8", "GBR", ""),
	}, ","))
	plan := scheduleWith(plannedPrice("usa-9", "USA"), plannedPrice("gbr-8", ""))

	var diags diag.Diagnostics
	(&subscriptionPriceScheduleResource{client: client}).write(context.Background(), plan, &diags)

	if diags.HasError() {
		t.Fatalf("write: %v", diags)
	}
	if got := len(ps.sent("PATCH /v1/subscriptions/sub-1")); got != 0 {
		t.Fatalf("sent %d PATCH, want none: Apple refuses an initial price once approved", got)
	}

	posts := ps.sent("POST /v1/subscriptionPrices")
	if len(posts) != 1 {
		t.Fatalf("sent %d POST, want 1 for the one territory that changed", len(posts))
	}

	var sent struct {
		Data struct {
			Attributes struct {
				StartDate *string `json:"startDate"`
			} `json:"attributes"`
			Relationships struct {
				Territory struct {
					Data struct{ ID string } `json:"data"`
				} `json:"territory"`
				SubscriptionPricePoint struct {
					Data struct{ ID string } `json:"data"`
				} `json:"subscriptionPricePoint"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(posts[0]), &sent); err != nil {
		t.Fatalf("decoding the POST: %v", err)
	}
	if got := sent.Data.Relationships.SubscriptionPricePoint.Data.ID; got != "usa-9" {
		t.Errorf("price point = %q, want usa-9", got)
	}
	if got := sent.Data.Relationships.Territory.Data.ID; got != "USA" {
		t.Errorf("territory = %q, want USA", got)
	}
	if sent.Data.Attributes.StartDate == nil || *sent.Data.Attributes.StartDate != "2026-11-28" {
		t.Errorf("startDate = %v, want 2026-11-28: an undated change is an initial price", sent.Data.Attributes.StartDate)
	}
}

// TestPriceScheduleWriteAfterApprovalDeletesAnUnwantedChange checks that a
// scheduled change the configuration does not name is withdrawn rather than
// stacked under a new one.
func TestPriceScheduleWriteAfterApprovalDeletesAnUnwantedChange(t *testing.T) {
	fixClock(t)
	ps, client := newPriceServer(t, "APPROVED", strings.Join([]string{
		apiPrice("p-now", "usa-8", "USA", ""),
		apiPrice("p-later", "usa-9", "USA", "2026-12-10"),
	}, ","))

	var diags diag.Diagnostics
	(&subscriptionPriceScheduleResource{client: client}).write(context.Background(), scheduleWith(plannedPrice("usa-8", "USA")), &diags)

	if diags.HasError() {
		t.Fatalf("write: %v", diags)
	}
	if got := len(ps.sent("DELETE /v1/subscriptionPrices/p-later")); got != 1 {
		t.Errorf("deleted the scheduled change %d times, want 1", got)
	}
	if got := len(ps.sent("POST /v1/subscriptionPrices")); got != 0 {
		t.Errorf("sent %d POST, want none: the price in effect is the one planned", got)
	}
}

// TestPriceScheduleWriteAfterApprovalNeedsATerritory checks that a new price
// point whose territory nothing reveals is refused before any request is sent.
func TestPriceScheduleWriteAfterApprovalNeedsATerritory(t *testing.T) {
	ps, client := newPriceServer(t, "APPROVED", apiPrice("p-usa", "usa-8", "USA", ""))

	var diags diag.Diagnostics
	(&subscriptionPriceScheduleResource{client: client}).write(context.Background(), scheduleWith(plannedPrice("usa-9", "")), &diags)

	if !diags.HasError() {
		t.Fatal("write succeeded, want an error naming the missing territory_id")
	}
	if !strings.Contains(diags[0].Detail(), "territory_id") {
		t.Errorf("error = %q, want it to name territory_id", diags[0].Detail())
	}
	if got := len(ps.sent("POST /v1/subscriptionPrices")); got != 0 {
		t.Errorf("sent %d POST, want none", got)
	}
}

// TestPriceScheduleWriteAfterApprovalWarnsAboutUnmanagedTerritories checks
// that a territory left out of the configuration is reported, not withdrawn.
func TestPriceScheduleWriteAfterApprovalWarnsAboutUnmanagedTerritories(t *testing.T) {
	ps, client := newPriceServer(t, "APPROVED", strings.Join([]string{
		apiPrice("p-usa", "usa-8", "USA", ""),
		apiPrice("p-gbr", "gbr-8", "GBR", ""),
	}, ","))

	var diags diag.Diagnostics
	(&subscriptionPriceScheduleResource{client: client}).write(context.Background(), scheduleWith(plannedPrice("usa-8", "USA")), &diags)

	if diags.HasError() {
		t.Fatalf("write: %v", diags)
	}
	if diags.WarningsCount() != 1 || !strings.Contains(diags[0].Detail(), "GBR") {
		t.Errorf("diagnostics = %v, want one warning naming GBR", diags)
	}
	for _, request := range ps.requests {
		if strings.HasPrefix(request, "DELETE") {
			t.Errorf("sent %s, want no deletion of a live price", request)
		}
	}
}
