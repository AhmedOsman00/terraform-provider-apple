// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package profile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestImportReportsTheIDLookupFailure pins which failures of the ID lookup
// fall back to a name lookup. Apple answers a name sent as an ID with a 404,
// so only a 404 does; a 500 used to fall back too and was reported as
// "profile with name ... not found", naming the import ID as the problem.
func TestImportReportsTheIDLookupFailure(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		status      int
		wantListed  bool
		wantInError string
	}{
		"server error is reported": {http.StatusInternalServerError, false, "status 500"},
		"404 falls back to name":   {http.StatusNotFound, true, "profile with name"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var listed atomic.Bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/v1/profiles" {
					listed.Store(true)
					_, _ = w.Write([]byte(`{"data":[],"links":{}}`))
					return
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(`{"errors":[{"status":"` + http.StatusText(test.status) + `","title":"Failure","detail":"canned"}]}`))
			}))
			defer srv.Close()

			ctx := context.Background()
			r := &profileResource{client: &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}}

			schemaResp := &resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
			resp := &resource.ImportStateResponse{State: tfsdk.State{
				Schema: schemaResp.Schema,
				Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
			}}

			r.ImportState(ctx, resource.ImportStateRequest{ID: "6Z5425CPMQ"}, resp)

			if !resp.Diagnostics.HasError() {
				t.Fatal("expected an error")
			}
			detail := resp.Diagnostics.Errors()[0].Detail()
			if !strings.Contains(detail, test.wantInError) {
				t.Errorf("error %q does not mention %q", detail, test.wantInError)
			}
			if listed.Load() != test.wantListed {
				t.Errorf("listed profiles = %v, want %v", listed.Load(), test.wantListed)
			}
		})
	}
}
