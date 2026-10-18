// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestReplaceReleasedVersion covers cutting the next release: a shipped
// version is replaced rather than renamed, which Apple refuses, while one still
// being prepared or in review is left to the in-place update.
func TestReplaceReleasedVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	NewAppStoreVersionResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)

	cases := map[string]bool{
		"READY_FOR_DISTRIBUTION":    true,
		"REPLACED_WITH_NEW_VERSION": true,
		"PENDING_DEVELOPER_RELEASE": true,
		"PREPARE_FOR_SUBMISSION":    false,
		"DEVELOPER_REJECTED":        false,
		"REJECTED":                  false,
		"WAITING_FOR_REVIEW":        false,
		"IN_REVIEW":                 false,
	}

	for appVersionState, want := range cases {
		t.Run(appVersionState, func(t *testing.T) {
			t.Parallel()

			state := tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
			}
			model := appStoreVersionResourceModel{}
			model.VersionString = types.StringValue("1.0.0")
			model.AppVersionState = types.StringValue(appVersionState)
			if diags := state.Set(ctx, &model); diags.HasError() {
				t.Fatalf("setting state: %v", diags)
			}

			req := planmodifier.StringRequest{
				Path:       path.Root("version_string"),
				State:      state,
				StateValue: types.StringValue("1.0.0"),
				PlanValue:  types.StringValue("1.1.0"),
			}
			resp := &stringplanmodifier.RequiresReplaceIfFuncResponse{}
			replaceReleasedVersion(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if resp.RequiresReplace != want {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, want)
			}
		})
	}
}

// TestVersionReleased covers the check a localization or screenshot set makes
// before deleting: a released version's records are only dropped from state.
func TestVersionReleased(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"READY_FOR_DISTRIBUTION": true,
		"PREPARE_FOR_SUBMISSION": false,
		"":                       false,
	}

	for appVersionState, want := range cases {
		t.Run(appVersionState, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/appStoreVersions/VERSION1" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				attributes := `{}`
				if appVersionState != "" {
					attributes = fmt.Sprintf(`{"appVersionState":%q}`, appVersionState)
				}
				fmt.Fprintf(w, `{"data":{"type":"appStoreVersions","id":"VERSION1","attributes":%s}}`, attributes)
			}))
			t.Cleanup(srv.Close)

			client := &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}
			got, err := versionReleased(client, "VERSION1")
			if err != nil {
				t.Fatalf("versionReleased returned error: %v", err)
			}
			if got != want {
				t.Errorf("versionReleased = %v, want %v", got, want)
			}
		})
	}
}
