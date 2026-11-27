// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package profile

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// profileAccount is a Developer Portal with three profiles for two App IDs.
// It counts the bundle ID lookups, which cost a request per profile.
func profileAccount(t *testing.T) (*profilesDataSource, func() int) {
	t.Helper()
	issuedFor := map[string]string{"prof-dev": "bundle-app", "prof-store": "bundle-app", "prof-widget": "bundle-widget"}

	var mu sync.Mutex
	lookups := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/profiles":
			var data []string
			for _, id := range []string{"prof-dev", "prof-store", "prof-widget"} {
				data = append(data, fmt.Sprintf(`{"type":"profiles","id":%q,"attributes":{"name":%q,"platform":"IOS","profileState":"ACTIVE","profileType":"IOS_APP_STORE"}}`, id, id))
			}
			fmt.Fprintf(w, `{"data":[%s],"links":{}}`, strings.Join(data, ","))
		case strings.HasSuffix(r.URL.Path, "/relationships/bundleId"):
			mu.Lock()
			lookups++
			mu.Unlock()
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/profiles/"), "/relationships/bundleId")
			fmt.Fprintf(w, `{"data":{"type":"bundleIds","id":%q}}`, issuedFor[id])
		case r.URL.Path == "/v1/bundleIds":
			fmt.Fprint(w, `{"data":[
				{"type":"bundleIds","id":"bundle-app","attributes":{"identifier":"com.example.app","name":"App","platform":"UNIVERSAL"}},
				{"type":"bundleIds","id":"bundle-widget","attributes":{"identifier":"com.example.app.widget","name":"Widget","platform":"UNIVERSAL"}}
			],"links":{}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"errors":[{"status":"404","title":"not found"}]}`)
		}
	}))
	t.Cleanup(srv.Close)

	ds := &profilesDataSource{client: &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}}
	return ds, func() int { mu.Lock(); defer mu.Unlock(); return lookups }
}

// readProfiles runs Read with the given filters and null everywhere else.
func readProfiles(t *testing.T, ds *profilesDataSource, values map[string]tftypes.Value) profilesDataSourceModel {
	t.Helper()
	ctx := context.Background()

	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	objectType, ok := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T", schemaResp.Schema.Type().TerraformType(ctx))
	}
	raw := map[string]tftypes.Value{}
	for name, attributeType := range objectType.AttributeTypes {
		raw[name] = tftypes.NewValue(attributeType, nil)
	}
	for name, value := range values {
		raw[name] = value
	}

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}}
	ds.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, raw)}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}

	var model profilesDataSourceModel
	if diags := resp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	return model
}

func profileIDs(model profilesDataSourceModel) string {
	var ids []string
	for _, p := range model.Profiles {
		ids = append(ids, p.ID.ValueString()+"="+p.BundleID.ValueString())
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

// TestProfilesDataSourceBundleIDFilter checks the filter that used to be
// accepted and ignored: profiles for another App ID came back, and every
// profile reported an empty bundle_id.
func TestProfilesDataSourceBundleIDFilter(t *testing.T) {
	str := func(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

	for name, tc := range map[string]struct {
		filter tftypes.Value
		want   string
	}{
		"by identifier":         {filter: str("com.example.app"), want: "prof-dev=bundle-app,prof-store=bundle-app"},
		"by Apple ID":           {filter: str("bundle-widget"), want: "prof-widget=bundle-widget"},
		"an unknown identifier": {filter: str("com.example.other"), want: ""},
		"no filter":             {filter: tftypes.NewValue(tftypes.String, nil), want: "prof-dev=bundle-app,prof-store=bundle-app,prof-widget=bundle-widget"},
	} {
		t.Run(name, func(t *testing.T) {
			ds, _ := profileAccount(t)
			if got := profileIDs(readProfiles(t, ds, map[string]tftypes.Value{"bundle_id": tc.filter})); got != tc.want {
				t.Errorf("profiles = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestProfilesDataSourceReadsEachBundleIDOnce checks the cost: one lookup per
// profile, shared between the filter and the reported bundle_id.
func TestProfilesDataSourceReadsEachBundleIDOnce(t *testing.T) {
	ds, lookups := profileAccount(t)
	readProfiles(t, ds, map[string]tftypes.Value{"bundle_id": tftypes.NewValue(tftypes.String, "bundle-app")})

	if got := lookups(); got != 3 {
		t.Errorf("looked up %d bundle IDs, want 3: one per profile", got)
	}
}
