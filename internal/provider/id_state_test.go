// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// idChangesOnWrite names the resources whose id is planned unknown on purpose,
// and why. Every other resource's id must keep its state on an update.
var idChangesOnWrite = map[string]string{
	"apple_app_availability":               "Apple issues a new record on every POST",
	"apple_app_price_schedule":             "Apple issues a new record on every POST",
	"apple_in_app_purchase_availability":   "Apple issues a new record on every POST",
	"apple_in_app_purchase_price_schedule": "Apple issues a new record on every POST",
	"apple_subscription_availability":      "Apple issues a new record on every POST",
	"apple_app_info":                       "the editable AppInfo is re-resolved on every write, and Apple issues a new one each version",
	"apple_app_age_rating_declaration":     "it hangs off the editable AppInfo, which Apple issues anew each version",
}

// TestResourceIDsKeepTheirState checks that a resource's id is not planned
// unknown by an update to something else. Another resource referencing it
// from an attribute that forces replacement would otherwise be replaced: an
// unknown value never equals the one in state. Renaming an apple_device
// reissued every apple_profile listing it this way.
func TestResourceIDsKeepTheirState(t *testing.T) {
	ctx := context.Background()

	for _, newResource := range New("test")().Resources(ctx) {
		r := newResource()

		metadata := &resource.MetadataResponse{}
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "apple"}, metadata)
		schemaResp := &resource.SchemaResponse{}
		r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

		id, ok := schemaResp.Schema.Attributes["id"].(schema.StringAttribute)
		if !ok {
			continue
		}

		keeps := false
		for _, modifier := range id.PlanModifiers {
			if strings.Contains(modifier.Description(ctx), "will not change") {
				keeps = true
			}
		}

		if _, exempt := idChangesOnWrite[metadata.TypeName]; exempt {
			if keeps {
				t.Errorf("%s.id keeps its state, so drop it from idChangesOnWrite", metadata.TypeName)
			}
			continue
		}
		if !keeps {
			t.Errorf("%s.id has no UseStateForUnknown: an update plans it unknown, which replaces anything "+
				"referencing it from an attribute that forces replacement", metadata.TypeName)
		}
	}
}
