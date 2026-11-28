// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// equalizedPoint is what each equalization read returns, reduced to what the
// tests compare: the point's ID and the territory it was decoded with.
type equalizedPoint struct {
	id, territory string
}

// equalizationReads are the in-app purchase and app equalization reads, which
// differ only in their path and their record type. The subscription read has
// its own tests in subscriptions_test.go.
var equalizationReads = []struct {
	name string
	path string
	kind string
	read func(c *Client, pricePointID string, territories []string) ([]equalizedPoint, error)
}{
	{
		name: "in-app purchase",
		path: "/v1/inAppPurchasePricePoints/base-usa/equalizations",
		kind: "inAppPurchasePricePoints",
		read: func(c *Client, id string, territories []string) ([]equalizedPoint, error) {
			points, err := c.GetInAppPurchasePricePointEqualizations(id, territories)
			out := make([]equalizedPoint, 0, len(points))
			for _, p := range points {
				point := equalizedPoint{id: p.ID}
				if p.Relationships != nil && p.Relationships.Territory != nil {
					point.territory = p.Relationships.Territory.Data.ID
				}
				out = append(out, point)
			}
			return out, err
		},
	},
	{
		name: "app",
		path: "/v3/appPricePoints/base-usa/equalizations",
		kind: "appPricePoints",
		read: func(c *Client, id string, territories []string) ([]equalizedPoint, error) {
			points, err := c.GetAppPricePointEqualizations(id, territories)
			out := make([]equalizedPoint, 0, len(points))
			for _, p := range points {
				point := equalizedPoint{id: p.ID}
				if p.Relationships != nil && p.Relationships.Territory != nil {
					point.territory = p.Relationships.Territory.Data.ID
				}
				out = append(out, point)
			}
			return out, err
		},
	},
}

// TestEqualizationReadsRequestTheEqualizationsPath covers the endpoint, the
// include that makes a returned point identifiable, and that an unfiltered read
// sends no territory filter.
func TestEqualizationReadsRequestTheEqualizationsPath(t *testing.T) {
	for _, tt := range equalizationReads {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotQuery url.Values

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.Query()
				fmt.Fprintf(w, `{"data":[
					{"type":%[1]q,"id":"eq-gbr",
						"attributes":{"customerPrice":"8.99","proceeds":"6.29"},
						"relationships":{"territory":{"data":{"type":"territories","id":"GBR"}}}},
					{"type":%[1]q,"id":"eq-egy",
						"attributes":{"customerPrice":"199.99","proceeds":"139.99"},
						"relationships":{"territory":{"data":{"type":"territories","id":"EGY"}}}}]}`, tt.kind)
			}))
			defer srv.Close()

			points, err := tt.read(newTestClient(srv), "base-usa", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotPath != tt.path {
				t.Errorf("path = %q, want %q", gotPath, tt.path)
			}
			if got := gotQuery.Get("include"); got != "territory" {
				t.Errorf("include = %q, want %q", got, "territory")
			}
			if _, present := gotQuery["filter[territory]"]; present {
				t.Error("filter[territory] was sent for an unfiltered request")
			}

			want := []equalizedPoint{{"eq-gbr", "GBR"}, {"eq-egy", "EGY"}}
			if len(points) != len(want) {
				t.Fatalf("got %d equalized price points, want %d", len(points), len(want))
			}
			for i := range want {
				if points[i] != want[i] {
					t.Errorf("point %d = %+v, want %+v", i, points[i], want[i])
				}
			}
		})
	}
}

// TestEqualizationReadsSendTerritoryFilter checks that a narrowed read passes
// the filter to Apple rather than discarding territories in memory.
func TestEqualizationReadsSendTerritoryFilter(t *testing.T) {
	for _, tt := range equalizationReads {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery url.Values

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.Query()
				fmt.Fprint(w, `{"data":[]}`)
			}))
			defer srv.Close()

			if _, err := tt.read(newTestClient(srv), "base-usa", []string{"GBR", "EGY"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := gotQuery.Get("filter[territory]"); got != "GBR,EGY" {
				t.Errorf("filter[territory] = %q, want %q", got, "GBR,EGY")
			}
		})
	}
}
