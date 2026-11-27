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

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// offerServer is an App Store Connect holding the introductory offers of one
// subscription, sub-1. Unlike priceServer it keeps state: a DELETE removes an
// offer, an end date PATCH changes one, and the subscription PATCH adds the
// offers it carries inline, so every read after a write sees what the write
// left behind, which is what recordCurrent depends on.
type offerServer struct {
	mu       sync.Mutex
	offers   []models.SubscriptionIntroductoryOffer
	requests []string
	bodies   map[string][]string
	created  int

	// failCreate makes the subscription PATCH answer 500. With
	// createBeforeFailing set, the offers are created first, the way a
	// response lost after Apple committed the write looks.
	failCreate          bool
	createBeforeFailing bool
	// missing makes every request about the subscription answer 404.
	missing bool
}

func newOfferServer(t *testing.T, offers ...models.SubscriptionIntroductoryOffer) (*offerServer, *subscriptionIntroductoryOfferScheduleResource) {
	t.Helper()
	os := &offerServer{offers: offers, bodies: map[string][]string{}}

	srv := httptest.NewServer(http.HandlerFunc(os.serve))
	t.Cleanup(srv.Close)

	client := &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}
	return os, &subscriptionIntroductoryOfferScheduleResource{client: client}
}

func (os *offerServer) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	key := r.Method + " " + r.URL.Path

	os.mu.Lock()
	defer os.mu.Unlock()
	os.requests = append(os.requests, key)
	os.bodies[key] = append(os.bodies[key], string(body))

	w.Header().Set("Content-Type", "application/json")
	notFound := func() {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errors":[{"status":"404","title":"The specified resource does not exist","detail":"not found"}]}`)
	}
	if os.missing {
		notFound()
		return
	}

	const offerPath = "/v1/subscriptionIntroductoryOffers/"
	switch {
	case key == "GET /v1/subscriptions/sub-1/introductoryOffers":
		data, _ := json.Marshal(os.offers)
		fmt.Fprintf(w, `{"data":%s,"links":{}}`, data)

	case key == "PATCH /v1/subscriptions/sub-1":
		if !os.failCreate || os.createBeforeFailing {
			os.addInline(body)
		}
		if os.failCreate {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"errors":[{"status":"500","title":"An unexpected error occurred."}]}`)
			return
		}
		fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{}}}`)

	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, offerPath):
		id := strings.TrimPrefix(r.URL.Path, offerPath)
		for i, offer := range os.offers {
			if offer.ID == id {
				os.offers = append(os.offers[:i], os.offers[i+1:]...)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		notFound()

	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, offerPath):
		id := strings.TrimPrefix(r.URL.Path, offerPath)
		var update struct {
			Data struct {
				Attributes struct {
					EndDate *string `json:"endDate"`
				} `json:"attributes"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &update)
		for i := range os.offers {
			if os.offers[i].ID == id {
				os.offers[i].Attributes.EndDate = update.Data.Attributes.EndDate
				data, _ := json.Marshal(os.offers[i])
				fmt.Fprintf(w, `{"data":%s}`, data)
				return
			}
		}
		notFound()

	default:
		notFound()
	}
}

// addInline creates the offers a subscription PATCH carries in included.
func (os *offerServer) addInline(body []byte) {
	var request struct {
		Included []struct {
			Attributes    models.SubscriptionIntroductoryOfferAttributes    `json:"attributes"`
			Relationships models.SubscriptionIntroductoryOfferRelationships `json:"relationships"`
		} `json:"included"`
	}
	_ = json.Unmarshal(body, &request)
	for _, inline := range request.Included {
		os.created++
		relationships := inline.Relationships
		os.offers = append(os.offers, models.SubscriptionIntroductoryOffer{
			Type:          "subscriptionIntroductoryOffers",
			ID:            fmt.Sprintf("new-%d", os.created),
			Attributes:    inline.Attributes,
			Relationships: &relationships,
		})
	}
}

func (os *offerServer) sent(key string) []string {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.bodies[key]
}

func (os *offerServer) writes() []string {
	os.mu.Lock()
	defer os.mu.Unlock()
	var writes []string
	for _, request := range os.requests {
		if !strings.HasPrefix(request, "GET ") {
			writes = append(writes, request)
		}
	}
	return writes
}

func (os *offerServer) held() []models.SubscriptionIntroductoryOffer {
	os.mu.Lock()
	defer os.mu.Unlock()
	return append([]models.SubscriptionIntroductoryOffer(nil), os.offers...)
}

// withEndDate returns the offer ending on the given date.
func withEndDate(offer models.SubscriptionIntroductoryOffer, date string) models.SubscriptionIntroductoryOffer {
	offer.Attributes.EndDate = &date
	return offer
}

// entryEnding returns the entry ending on the given date.
func entryEnding(entry introductoryOfferValues, date string) introductoryOfferValues {
	entry.EndDate = types.StringValue(date)
	return entry
}

func offerSchedule(id types.String, offers ...introductoryOfferValues) subscriptionIntroductoryOfferScheduleModel {
	return subscriptionIntroductoryOfferScheduleModel{
		ID:             id,
		SubscriptionID: types.StringValue("sub-1"),
		Offers:         offers,
	}
}

func offerScheduleSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&subscriptionIntroductoryOfferScheduleResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp.Schema
}

// offerState renders a model as the framework would hand it to the resource.
func offerState(t *testing.T, s schema.Schema, model subscriptionIntroductoryOfferScheduleModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, model); diags.HasError() {
		t.Fatalf("building state: %v", diags)
	}
	return state
}

func emptyOfferState(s schema.Schema) tfsdk.State {
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}
}

func offersIn(t *testing.T, state tfsdk.State) subscriptionIntroductoryOfferScheduleModel {
	t.Helper()
	var model subscriptionIntroductoryOfferScheduleModel
	if diags := state.Get(context.Background(), &model); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	return model
}

// territoriesOf lists the territory of each entry, in order.
func territoriesOf(entries []introductoryOfferValues) []string {
	territories := make([]string, 0, len(entries))
	for _, entry := range entries {
		territories = append(territories, entry.TerritoryID.ValueString()+"/"+entry.Duration.ValueString())
	}
	return territories
}

// update runs Update from prior to plan.
func (r *subscriptionIntroductoryOfferScheduleResource) testUpdate(t *testing.T, prior, plan subscriptionIntroductoryOfferScheduleModel) resource.UpdateResponse {
	t.Helper()
	s := offerScheduleSchema(t)
	priorState := offerState(t, s, prior)
	planned := offerState(t, s, plan)

	resp := resource.UpdateResponse{State: planned}
	r.Update(context.Background(), resource.UpdateRequest{
		Plan:  tfsdk.Plan{Schema: s, Raw: planned.Raw},
		State: priorState,
	}, &resp)
	return resp
}

// TestOfferScheduleUpdateReplacesAnOfferInItsTerritory checks the order Apple
// forces: a territory holds one offer at a time, so the old one is deleted
// before the new one is created, and the create names only the new offer.
// Naming a kept offer there makes Apple answer 500.
func TestOfferScheduleUpdateReplacesAnOfferInItsTerritory(t *testing.T) {
	os, r := newOfferServer(t,
		apiOffer("live-usa", "USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""),
		apiOffer("live-gbr", "GBR", models.SubscriptionOfferModeFreeTrial, "ONE_MONTH", 1, ""),
	)
	gbr := offerEntry("GBR", models.SubscriptionOfferModeFreeTrial, "ONE_MONTH", 1, "")
	prior := offerSchedule(types.StringValue("sub-1"), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""), gbr)
	plan := offerSchedule(types.StringValue("sub-1"), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "TWO_WEEKS", 1, ""), gbr)

	resp := r.testUpdate(t, prior, plan)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	want := []string{"DELETE /v1/subscriptionIntroductoryOffers/live-usa", "PATCH /v1/subscriptions/sub-1"}
	if got := os.writes(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("writes = %v, want %v: the old offer has to go before its replacement is created", got, want)
	}

	var sent struct {
		Data struct {
			Relationships struct {
				IntroductoryOffers struct {
					Data []models.ResourceData `json:"data"`
				} `json:"introductoryOffers"`
			} `json:"relationships"`
		} `json:"data"`
		Included []json.RawMessage `json:"included"`
	}
	if err := json.Unmarshal([]byte(os.sent("PATCH /v1/subscriptions/sub-1")[0]), &sent); err != nil {
		t.Fatalf("decoding the PATCH: %v", err)
	}
	if linkage := sent.Data.Relationships.IntroductoryOffers.Data; len(linkage) != 1 || linkage[0].ID != "${offer0}" {
		t.Errorf("introductoryOffers = %v, want only the new offer's placeholder: naming live-gbr makes Apple answer 500", linkage)
	}
	if len(sent.Included) != 1 {
		t.Errorf("included %d offers, want 1", len(sent.Included))
	}

	if got := territoriesOf(offersIn(t, resp.State).Offers); strings.Join(got, ",") != "USA/TWO_WEEKS,GBR/ONE_MONTH" {
		t.Errorf("state offers = %v, want the plan", got)
	}
	if held := os.held(); len(held) != 2 {
		t.Errorf("Apple holds %d offers, want 2", len(held))
	}
}

// TestOfferScheduleUpdateClearsOnlyAManagedEndDate checks that removing an end
// date sends an explicit null for an offer whose end date state held, and
// leaves alone one whose end date state never knew: Read does not adopt an end
// date the configuration left unset, so clearing it would change something no
// plan showed.
func TestOfferScheduleUpdateClearsOnlyAManagedEndDate(t *testing.T) {
	os, r := newOfferServer(t,
		withEndDate(apiOffer("live-usa", "USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""), "2027-01-31"),
		withEndDate(apiOffer("live-gbr", "GBR", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""), "2027-02-28"),
	)
	usa := offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, "")
	gbr := offerEntry("GBR", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, "")
	prior := offerSchedule(types.StringValue("sub-1"), entryEnding(usa, "2027-01-31"), gbr)
	plan := offerSchedule(types.StringValue("sub-1"), usa, gbr)

	resp := r.testUpdate(t, prior, plan)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	want := []string{"PATCH /v1/subscriptionIntroductoryOffers/live-usa"}
	if got := os.writes(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("writes = %v, want %v", got, want)
	}
	if body := os.sent(want[0])[0]; !strings.Contains(body, `"endDate":null`) {
		t.Errorf("PATCH body = %s, want an explicit null end date: omitting it leaves the date in place", body)
	}
	for _, offer := range os.held() {
		switch offer.ID {
		case "live-usa":
			if offer.Attributes.EndDate != nil {
				t.Errorf("live-usa still ends %s", *offer.Attributes.EndDate)
			}
		case "live-gbr":
			if offer.Attributes.EndDate == nil || *offer.Attributes.EndDate != "2027-02-28" {
				t.Errorf("live-gbr end date = %v, want it untouched", offer.Attributes.EndDate)
			}
		}
	}
}

// TestOfferScheduleUpdateRecordsWhatAppleHoldsAfterAFailedCreate checks the
// state written when the create fails after a deletion went through. Apple
// then holds fewer offers than either the plan or the prior state, and state
// has to say so, or the next plan would not show the missing offer.
func TestOfferScheduleUpdateRecordsWhatAppleHoldsAfterAFailedCreate(t *testing.T) {
	os, r := newOfferServer(t,
		apiOffer("live-usa", "USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""),
		apiOffer("live-gbr", "GBR", models.SubscriptionOfferModeFreeTrial, "ONE_MONTH", 1, ""),
	)
	os.failCreate = true
	gbr := offerEntry("GBR", models.SubscriptionOfferModeFreeTrial, "ONE_MONTH", 1, "")
	prior := offerSchedule(types.StringValue("sub-1"), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""), gbr)
	plan := offerSchedule(types.StringValue("sub-1"), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "TWO_WEEKS", 1, ""), gbr)

	resp := r.testUpdate(t, prior, plan)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Update succeeded, want the create's error")
	}
	if got := territoriesOf(offersIn(t, resp.State).Offers); strings.Join(got, ",") != "GBR/ONE_MONTH" {
		t.Errorf("state offers = %v, want only GBR: USA's old offer is deleted and its new one was never created", got)
	}
}

// TestOfferScheduleUpdateDropsStateWhenNothingIsLeft checks that a failed
// write which leaves the subscription with no offers records the schedule as
// gone, the way Read would find it, rather than an empty set.
func TestOfferScheduleUpdateDropsStateWhenNothingIsLeft(t *testing.T) {
	os, r := newOfferServer(t, apiOffer("live-usa", "USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""))
	os.failCreate = true
	prior := offerSchedule(types.StringValue("sub-1"), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""))
	plan := offerSchedule(types.StringValue("sub-1"), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "TWO_WEEKS", 1, ""))

	resp := r.testUpdate(t, prior, plan)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Update succeeded, want the create's error")
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("state = %v, want none: Apple holds no offers", resp.State.Raw)
	}
}

// TestOfferScheduleCreateRecordsOffersAppleCreatedBeforeFailing checks a 500
// that arrives after Apple committed the create. The offers exist, so they
// have to be in state, or the next apply would refuse the subscription for
// already having offers.
func TestOfferScheduleCreateRecordsOffersAppleCreatedBeforeFailing(t *testing.T) {
	os, r := newOfferServer(t)
	os.failCreate, os.createBeforeFailing = true, true
	s := offerScheduleSchema(t)
	planned := offerState(t, s, offerSchedule(types.StringUnknown(), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, "")))

	resp := resource.CreateResponse{State: emptyOfferState(s)}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planned.Raw}}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Create succeeded, want the 500")
	}
	recorded := offersIn(t, resp.State)
	if recorded.ID.ValueString() != "sub-1" {
		t.Errorf("id = %v, want sub-1", recorded.ID)
	}
	if got := territoriesOf(recorded.Offers); strings.Join(got, ",") != "USA/ONE_WEEK" {
		t.Errorf("state offers = %v, want the offer Apple created", got)
	}
}

// TestOfferScheduleCreateWritesThePlan checks the ordinary create: one PATCH
// carrying every offer, and the plan recorded with the subscription as its ID.
func TestOfferScheduleCreateWritesThePlan(t *testing.T) {
	os, r := newOfferServer(t)
	s := offerScheduleSchema(t)
	planned := offerState(t, s, offerSchedule(types.StringUnknown(),
		offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""),
		offerEntry("GBR", models.SubscriptionOfferModePayAsYouGo, "ONE_MONTH", 3, "gbr-1"),
	))

	resp := resource.CreateResponse{State: emptyOfferState(s)}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planned.Raw}}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if got := os.writes(); len(got) != 1 || got[0] != "PATCH /v1/subscriptions/sub-1" {
		t.Errorf("writes = %v, want one subscription PATCH", got)
	}
	if held := os.held(); len(held) != 2 {
		t.Errorf("Apple holds %d offers, want 2", len(held))
	}
	if got := offersIn(t, resp.State).ID.ValueString(); got != "sub-1" {
		t.Errorf("id = %q, want sub-1", got)
	}
}

// TestOfferScheduleDeleteRemovesOnlyTheOffersStateNames checks that destroy
// deletes what state lists and nothing else on the subscription.
func TestOfferScheduleDeleteRemovesOnlyTheOffersStateNames(t *testing.T) {
	os, r := newOfferServer(t,
		apiOffer("live-usa", "USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""),
		apiOffer("live-gbr", "GBR", models.SubscriptionOfferModeFreeTrial, "ONE_MONTH", 1, ""),
	)
	s := offerScheduleSchema(t)
	state := offerState(t, s, offerSchedule(types.StringValue("sub-1"), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, "")))

	var resp resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete: %v", resp.Diagnostics)
	}
	if got := os.writes(); len(got) != 1 || got[0] != "DELETE /v1/subscriptionIntroductoryOffers/live-usa" {
		t.Errorf("writes = %v, want only live-usa deleted", got)
	}
}

// TestOfferScheduleDeleteToleratesADeletedSubscription checks that destroying
// a schedule whose subscription is already gone succeeds: the offers went with
// it.
func TestOfferScheduleDeleteToleratesADeletedSubscription(t *testing.T) {
	os, r := newOfferServer(t)
	os.missing = true
	s := offerScheduleSchema(t)
	state := offerState(t, s, offerSchedule(types.StringValue("sub-1"), offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, "")))

	var resp resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete: %v", resp.Diagnostics)
	}
}

// TestOfferScheduleRead covers the three things a refresh can find.
func TestOfferScheduleRead(t *testing.T) {
	usa := offerEntry("USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, "")

	cases := []struct {
		name    string
		offers  []models.SubscriptionIntroductoryOffer
		missing bool
		want    string // territories in state, "" for removed
	}{
		{
			name:    "a deleted subscription removes the schedule",
			missing: true,
		},
		{
			name: "no offers left removes the schedule",
		},
		{
			name: "an offer created by hand is added",
			offers: []models.SubscriptionIntroductoryOffer{
				apiOffer("live-usa", "USA", models.SubscriptionOfferModeFreeTrial, "ONE_WEEK", 1, ""),
				apiOffer("live-gbr", "GBR", models.SubscriptionOfferModeFreeTrial, "ONE_MONTH", 1, ""),
			},
			want: "USA/ONE_WEEK,GBR/ONE_MONTH",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os, r := newOfferServer(t, tc.offers...)
			os.missing = tc.missing
			s := offerScheduleSchema(t)
			state := offerState(t, s, offerSchedule(types.StringValue("sub-1"), usa))

			resp := resource.ReadResponse{State: state}
			r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			if tc.want == "" {
				if !resp.State.Raw.IsNull() {
					t.Errorf("state = %v, want it removed", resp.State.Raw)
				}
				return
			}
			if got := territoriesOf(offersIn(t, resp.State).Offers); strings.Join(got, ",") != tc.want {
				t.Errorf("state offers = %v, want %s", got, tc.want)
			}
		})
	}
}
