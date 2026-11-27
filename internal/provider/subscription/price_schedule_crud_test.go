// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package subscription

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func priceScheduleSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&subscriptionPriceScheduleResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp.Schema
}

func priceState(t *testing.T, s schema.Schema, model *subscriptionPriceScheduleModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, model); diags.HasError() {
		t.Fatalf("building state: %v", diags)
	}
	return state
}

func pricesIn(t *testing.T, state tfsdk.State) subscriptionPriceScheduleModel {
	t.Helper()
	var model subscriptionPriceScheduleModel
	if diags := state.Get(context.Background(), &model); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	return model
}

// schedulePricePoints lists each price's point, with its start date when one is set.
func schedulePricePoints(prices []subscriptionSchedulePriceModel) string {
	points := make([]string, 0, len(prices))
	for _, price := range prices {
		point := price.PricePointID.ValueString()
		if !price.StartDate.IsNull() {
			point += "@" + price.StartDate.ValueString()
		}
		points = append(points, point)
	}
	return strings.Join(points, ",")
}

// listedPreservedPrice renders a price Apple keeps for existing subscribers.
func listedPreservedPrice(id, point, territory, startDate string) string {
	return strings.Replace(apiPrice(id, point, territory, startDate), `"attributes":{`, `"attributes":{"preserved":true,`, 1)
}

// TestPriceScheduleRead covers what a refresh finds after the writes the
// resource makes, including the dated change it sends for an approved
// subscription: Apple then lists the old price beside the scheduled one, and a
// configuration asking for the new one must plan nothing.
func TestPriceScheduleRead(t *testing.T) {
	scheduled := scheduleWith(plannedPrice("usa-9", "USA"))
	scheduled.ID = types.StringValue("sub-1")

	cases := []struct {
		name    string
		prices  []string
		missing bool
		want    string // price points in state, "" for removed
	}{
		{
			name:    "a deleted subscription removes the schedule",
			missing: true,
		},
		{
			name: "no prices removes the schedule",
		},
		{
			name: "a scheduled change reads back as the configured price",
			prices: []string{
				apiPrice("p-now", "usa-8", "USA", ""),
				apiPrice("p-later", "usa-9", "USA", "2026-11-28"),
			},
			want: "usa-9",
		},
		{
			name: "a preserved price is not the price in effect",
			prices: []string{
				apiPrice("p-now", "usa-9", "USA", "2026-11-28"),
				listedPreservedPrice("p-kept", "usa-8", "USA", "2026-12-01"),
			},
			want: "usa-9",
		},
		{
			name: "a price changed by hand replaces the configured one",
			prices: []string{
				apiPrice("p-now", "usa-9", "USA", ""),
				apiPrice("p-hand", "usa-10", "USA", "2026-12-10"),
			},
			want: "usa-10",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ps, client := newPriceServer(t, "APPROVED", strings.Join(tc.prices, ","))
			ps.missing = tc.missing
			s := priceScheduleSchema(t)
			state := priceState(t, s, scheduled)

			resp := resource.ReadResponse{State: state}
			(&subscriptionPriceScheduleResource{client: client}).Read(context.Background(), resource.ReadRequest{State: state}, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			if tc.want == "" {
				if !resp.State.Raw.IsNull() {
					t.Errorf("state = %v, want it removed", resp.State.Raw)
				}
				return
			}
			if got := schedulePricePoints(pricesIn(t, resp.State).Prices); got != tc.want {
				t.Errorf("state prices = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestPriceScheduleWriteNamesTheCauseApplesErrorHides checks the diagnostics
// for the refusals whose text says nothing useful: the missing availability
// behind "processing the pricing information", and a subscription in review.
func TestPriceScheduleWriteNamesTheCauseApplesErrorHides(t *testing.T) {
	cases := []struct {
		name    string
		state   string
		prices  string
		refuse  string
		message string
		want    string
	}{
		{
			name:    "no availability before the initial price",
			state:   "MISSING_METADATA",
			refuse:  "PATCH /v1/subscriptions/sub-1",
			message: "An error occurred while processing the pricing information",
			want:    "apple_subscription_availability",
		},
		{
			name:    "a price change while in review",
			state:   "WAITING_FOR_REVIEW",
			prices:  apiPrice("p-usa", "usa-8", "USA", ""),
			refuse:  "POST /v1/subscriptionPrices",
			message: "You cannot change prices while the subscription is in review",
			want:    "in review",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixClock(t)
			ps, client := newPriceServer(t, tc.state, tc.prices)
			ps.refuse[tc.refuse] = tc.message

			var diags diag.Diagnostics
			plan := scheduleWith(plannedPrice("usa-9", "USA"))
			(&subscriptionPriceScheduleResource{client: client}).write(context.Background(), plan, &diags)

			if !diags.HasError() {
				t.Fatal("write succeeded, want Apple's refusal")
			}
			if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, tc.want) {
				t.Errorf("error = %q, want it to name %q", detail, tc.want)
			}
			if !plan.ID.IsUnknown() {
				t.Errorf("id = %v, want it left unknown after a failed write", plan.ID)
			}
		})
	}
}

// TestPriceScheduleDeleteLeavesThePricesAtApple checks that destroy only
// warns: a live price cannot be withdrawn, only superseded.
func TestPriceScheduleDeleteLeavesThePricesAtApple(t *testing.T) {
	ps, client := newPriceServer(t, "APPROVED", apiPrice("p-usa", "usa-8", "USA", ""))
	s := priceScheduleSchema(t)
	state := priceState(t, s, scheduleWith(plannedPrice("usa-8", "USA")))

	var resp resource.DeleteResponse
	(&subscriptionPriceScheduleResource{client: client}).Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)

	if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
		t.Errorf("diagnostics = %v, want one warning", resp.Diagnostics)
	}
	if len(ps.requests) != 0 {
		t.Errorf("Delete sent %v, want nothing", ps.requests)
	}
}

// TestPriceScheduleImport checks that an import adopts the price in effect per
// territory and refuses a subscription with nothing to adopt.
func TestPriceScheduleImport(t *testing.T) {
	t.Run("adopts the price in effect", func(t *testing.T) {
		_, client := newPriceServer(t, "APPROVED", strings.Join([]string{
			apiPrice("p-old", "usa-7", "USA", "2026-01-01"),
			apiPrice("p-now", "usa-8", "USA", "2026-06-01"),
			listedPreservedPrice("p-kept", "usa-6", "USA", "2026-06-01"),
		}, ","))
		s := priceScheduleSchema(t)

		resp := resource.ImportStateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}}
		(&subscriptionPriceScheduleResource{client: client}).ImportState(context.Background(), resource.ImportStateRequest{ID: "sub-1"}, &resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("ImportState: %v", resp.Diagnostics)
		}
		imported := pricesIn(t, resp.State)
		if imported.ID.ValueString() != "sub-1" || imported.SubscriptionID.ValueString() != "sub-1" {
			t.Errorf("id = %v, subscription_id = %v, want both sub-1", imported.ID, imported.SubscriptionID)
		}
		if got := schedulePricePoints(imported.Prices); !strings.HasPrefix(got, "usa-8") || strings.Contains(got, ",") {
			t.Errorf("imported prices = %s, want only usa-8: the history and the preserved price are not in effect", got)
		}
	})

	t.Run("refuses a subscription with no prices", func(t *testing.T) {
		_, client := newPriceServer(t, "APPROVED", "")
		s := priceScheduleSchema(t)

		resp := resource.ImportStateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}}
		(&subscriptionPriceScheduleResource{client: client}).ImportState(context.Background(), resource.ImportStateRequest{ID: "sub-1"}, &resp)

		if errs := resp.Diagnostics.Errors(); len(errs) != 1 || errs[0].Summary() != "Subscription Has No Prices" {
			t.Errorf("diagnostics = %v, want 'Subscription Has No Prices'", resp.Diagnostics)
		}
	})
}
