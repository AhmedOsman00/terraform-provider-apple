// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// TestGetSubscriptionPricePointsSendsFilters covers the query parameters the
// price point catalogue depends on. Without include=territory a returned price
// point cannot be identified, and without filter[territory] the request pulls
// every territory Apple sells in.
func TestGetSubscriptionPricePointsSendsFilters(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"type":"subscriptionPricePoints","id":"p1",
			"attributes":{"customerPrice":"9.99","proceeds":"6.99"},
			"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}]}`)
	}))
	defer srv.Close()

	points, err := newTestClient(srv).GetSubscriptionPricePoints("sub1", []string{"USA", "GBR"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("include"); got != "territory" {
		t.Errorf("include = %q, want %q", got, "territory")
	}
	if got := gotQuery.Get("filter[territory]"); got != "USA,GBR" {
		t.Errorf("filter[territory] = %q, want %q", got, "USA,GBR")
	}
	if got := gotQuery.Get("limit"); got != "200" {
		t.Errorf("limit = %q, want %q", got, "200")
	}

	if len(points) != 1 {
		t.Fatalf("got %d price points, want 1", len(points))
	}
	if points[0].Relationships == nil || points[0].Relationships.Territory == nil {
		t.Fatal("territory relationship was not decoded")
	}
	if got := points[0].Relationships.Territory.Data.ID; got != "USA" {
		t.Errorf("territory = %q, want %q", got, "USA")
	}
}

// TestGetSubscriptionPricePointsOmitsEmptyTerritoryFilter checks that an
// unfiltered call sends no filter at all rather than an empty one, which Apple
// would reject.
func TestGetSubscriptionPricePointsOmitsEmptyTerritoryFilter(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).GetSubscriptionPricePoints("sub1", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, present := gotQuery["filter[territory]"]; present {
		t.Error("filter[territory] was sent for an unfiltered request")
	}
}

// TestPaginationCarriesQueryParametersAcrossPages covers the interaction
// between getAllPagesQuery and the "next" link: Apple's link already carries
// the parameters forward, so they must not be re-appended and duplicated.
func TestPaginationCarriesQueryParametersAcrossPages(t *testing.T) {
	var requested []string

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.String())

		if r.URL.Query().Get("cursor") == "" {
			fmt.Fprintf(w, `{"data":[{"type":"subscriptionPrices","id":"a"}],
				"links":{"next":"%s/v1/subscriptions/sub1/prices?include=territory%%2CsubscriptionPricePoint&limit=200&cursor=next"}}`, srv.URL)
			return
		}
		fmt.Fprint(w, `{"data":[{"type":"subscriptionPrices","id":"b"}]}`)
	}))
	defer srv.Close()

	prices, err := newTestClient(srv).GetSubscriptionPrices("sub1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prices) != 2 {
		t.Fatalf("got %d prices, want 2 -- pagination did not follow the next link", len(prices))
	}
	if len(requested) != 2 {
		t.Fatalf("made %d requests, want 2", len(requested))
	}

	// The first request must carry both include values, and the second must be
	// exactly the link Apple supplied.
	first := requested[0]
	for _, want := range []string{"include=territory%2CsubscriptionPricePoint", "limit=200"} {
		if !strings.Contains(first, want) {
			t.Errorf("first request %q does not contain %q", first, want)
		}
	}
	if !strings.Contains(requested[1], "cursor=next") {
		t.Errorf("second request %q did not follow the next link", requested[1])
	}
}

// TestGetSubscriptionRequestsGroupInclude covers the include that makes import
// possible: without it Apple reports the group relationship as links alone.
func TestGetSubscriptionRequestsGroupInclude(t *testing.T) {
	var gotQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub1",
			"attributes":{"name":"Pro","productId":"com.example.pro","state":"MISSING_METADATA"},
			"relationships":{"group":{"data":{"type":"subscriptionGroups","id":"grp1"}}}}}`)
	}))
	defer srv.Close()

	subscription, err := newTestClient(srv).GetSubscription("sub1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("include"); got != "group" {
		t.Errorf("include = %q, want %q", got, "group")
	}
	if subscription.Relationships == nil || subscription.Relationships.Group == nil {
		t.Fatal("group relationship was not decoded")
	}
	if got := subscription.Relationships.Group.Data.ID; got != "grp1" {
		t.Errorf("group = %q, want %q", got, "grp1")
	}
}

// TestCreateSubscriptionOmitsUnsetAttributes checks that optional attributes
// left unset are absent from the request body rather than sent as zero values.
// Apple treats an explicit null differently from an omitted member.
func TestCreateSubscriptionOmitsUnsetAttributes(t *testing.T) {
	var body map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("could not decode request body: %v", err)
		}
		fmt.Fprint(w, `{"data":{"type":"subscriptions","id":"sub1",
			"attributes":{"name":"Pro","productId":"com.example.pro"}}}`)
	}))
	defer srv.Close()

	attributes := models.SubscriptionCreateAttributes{
		Name:      "Pro",
		ProductID: "com.example.pro",
	}

	_, err := newTestClient(srv).CreateSubscription("grp1", attributes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := body["data"].(map[string]interface{})
	sentAttributes, _ := data["attributes"].(map[string]interface{})

	for _, absent := range []string{"familySharable", "subscriptionPeriod", "reviewNote", "groupLevel"} {
		if _, present := sentAttributes[absent]; present {
			t.Errorf("attribute %q was sent despite being unset", absent)
		}
	}
	if got := sentAttributes["productId"]; got != "com.example.pro" {
		t.Errorf("productId = %v, want com.example.pro", got)
	}

	relationships, _ := data["relationships"].(map[string]interface{})
	group, _ := relationships["group"].(map[string]interface{})
	groupData, _ := group["data"].(map[string]interface{})
	if got := groupData["id"]; got != "grp1" {
		t.Errorf("group id = %v, want grp1", got)
	}
	if got := groupData["type"]; got != "subscriptionGroups" {
		t.Errorf("group type = %v, want subscriptionGroups", got)
	}
}

// TestCreateSubscriptionPriceOmitsEmptyAttributes pins the wire format.
//
// Every attribute is optional, so a price with no start date, no
// preserveCurrentPrice and no plan type marshalled to "attributes":{} — and
// App Store Connect answered 409 "An error occurred while processing the
// pricing information", which reads like a complaint about the price point.
// The member has to be absent, not empty.
func TestCreateSubscriptionPriceOmitsEmptyAttributes(t *testing.T) {
	var body map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %s", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"type":"subscriptionPrices","id":"price-1","attributes":{}}}`)
	}))
	defer srv.Close()

	client := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}

	if _, err := client.CreateSubscriptionPrice(
		"sub-1", "point-1", nil, &models.SubscriptionPriceCreateAttributes{}, nil,
	); err != nil {
		t.Fatalf("CreateSubscriptionPrice: %s", err)
	}

	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("request has no data member: %v", body)
	}
	if _, present := data["attributes"]; present {
		t.Errorf("attributes member sent for a price with nothing set: %v", data["attributes"])
	}

	// A price that does set something must still carry it.
	startDate := "2027-01-01"
	if _, err := client.CreateSubscriptionPrice(
		"sub-1", "point-1", nil,
		&models.SubscriptionPriceCreateAttributes{StartDate: &startDate}, nil,
	); err != nil {
		t.Fatalf("CreateSubscriptionPrice with a start date: %s", err)
	}

	data, _ = body["data"].(map[string]any)
	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes dropped for a price that sets a start date: %v", data)
	}
	if attrs["startDate"] != startDate {
		t.Errorf("startDate = %v, want %q", attrs["startDate"], startDate)
	}
}
