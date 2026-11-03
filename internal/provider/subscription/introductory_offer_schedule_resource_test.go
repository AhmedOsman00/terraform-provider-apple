// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// scheduleTypes returns the schedule's schema and the Terraform types of the
// whole resource and of one offer in it.
func scheduleTypes(t *testing.T) (resource.SchemaResponse, tftypes.Object, tftypes.Object) {
	t.Helper()
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	(&subscriptionIntroductoryOfferScheduleResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	objectType, ok := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want tftypes.Object", schemaResp.Schema.Type().TerraformType(ctx))
	}
	offersType, ok := objectType.AttributeTypes["offers"].(tftypes.Set)
	if !ok {
		t.Fatalf("offers type is %T, want tftypes.Set", objectType.AttributeTypes["offers"])
	}
	offerType, ok := offersType.ElementType.(tftypes.Object)
	if !ok {
		t.Fatalf("offer type is %T, want tftypes.Object", offersType.ElementType)
	}
	return schemaResp, objectType, offerType
}

// scheduleOffer builds one free trial, with a price point when one is given.
func scheduleOffer(offerType tftypes.Object, territory, duration string, pricePoint *string) tftypes.Value {
	point := tftypes.NewValue(tftypes.String, nil)
	if pricePoint != nil {
		point = tftypes.NewValue(tftypes.String, *pricePoint)
	}
	return tftypes.NewValue(offerType, map[string]tftypes.Value{
		"territory_id":                  tftypes.NewValue(tftypes.String, territory),
		"offer_mode":                    tftypes.NewValue(tftypes.String, "FREE_TRIAL"),
		"duration":                      tftypes.NewValue(tftypes.String, duration),
		"number_of_periods":             tftypes.NewValue(tftypes.Number, 1),
		"price_point_id":                point,
		"start_date":                    tftypes.NewValue(tftypes.String, nil),
		"end_date":                      tftypes.NewValue(tftypes.String, nil),
		"target_subscription_plan_type": tftypes.NewValue(tftypes.String, nil),
	})
}

// scheduleValue builds the whole resource for one subscription.
func scheduleValue(objectType tftypes.Object, id any, offers ...tftypes.Value) tftypes.Value {
	return tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id":              tftypes.NewValue(tftypes.String, id),
		"subscription_id": tftypes.NewValue(tftypes.String, "sub-1"),
		"offers":          tftypes.NewValue(objectType.AttributeTypes["offers"], offers),
	})
}

// TestIntroductoryOfferScheduleValidateConfigReportsEveryOffer checks that one
// offer's validation error does not stop the rest from being checked: a
// configuration with two mistakes reports both in the same plan.
func TestIntroductoryOfferScheduleValidateConfigReportsEveryOffer(t *testing.T) {
	ctx := context.Background()
	schemaResp, objectType, offerType := scheduleTypes(t)

	// A free trial carrying a price point in USA, and two undated offers in GBR.
	pricePoint := "point-usa"
	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw: scheduleValue(objectType, nil,
			scheduleOffer(offerType, "USA", "ONE_WEEK", &pricePoint),
			scheduleOffer(offerType, "GBR", "ONE_WEEK", nil),
			scheduleOffer(offerType, "GBR", "TWO_WEEKS", nil),
		),
	}

	var resp resource.ValidateConfigResponse
	(&subscriptionIntroductoryOfferScheduleResource{}).ValidateConfig(ctx, resource.ValidateConfigRequest{Config: config}, &resp)

	summaries := map[string]bool{}
	for _, d := range resp.Diagnostics.Errors() {
		summaries[d.Summary()] = true
	}
	for _, want := range []string{"Price Point Does Not Fit The Offer Mode", "Overlapping Introductory Offers"} {
		if !summaries[want] {
			t.Errorf("missing %q among %v", want, summaries)
		}
	}
}

// TestIntroductoryOfferScheduleCreateRefusesExistingOffers checks that Create
// writes nothing to a subscription that already has offers. A write deletes
// every offer the plan does not match, and a plan for a create shows only an
// addition, so those offers have to be imported instead.
func TestIntroductoryOfferScheduleCreateRefusesExistingOffers(t *testing.T) {
	ctx := context.Background()
	schemaResp, objectType, offerType := scheduleTypes(t)

	var writes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes = append(writes, r.Method+" "+r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"type":"subscriptionIntroductoryOffers","id":"live-1",
			"attributes":{"offerMode":"FREE_TRIAL","duration":"TWO_MONTHS","numberOfPeriods":1},
			"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}]}`)
	}))
	defer srv.Close()

	r := &subscriptionIntroductoryOfferScheduleResource{
		client: &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"},
	}

	plan := tfsdk.Plan{
		Schema: schemaResp.Schema,
		Raw: scheduleValue(objectType, tftypes.UnknownValue,
			scheduleOffer(offerType, "USA", "ONE_WEEK", nil),
		),
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)

	errors := resp.Diagnostics.Errors()
	if len(errors) != 1 || errors[0].Summary() != "Subscription Already Has Introductory Offers" {
		t.Fatalf("diagnostics = %v, want one 'Subscription Already Has Introductory Offers' error", resp.Diagnostics)
	}
	if len(writes) != 0 {
		t.Errorf("Create sent %v, want no writes", writes)
	}
	if !resp.State.Raw.IsNull() {
		t.Errorf("Create recorded state %v, want none", resp.State.Raw)
	}
}
