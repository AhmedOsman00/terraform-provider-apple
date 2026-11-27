// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"16.10", "16.2", 1},
		{"16.2", "16.10", -1},
		{"16", "16.0.1", -1},
		{"26.0", "26.0", 0},
		{"16.4 beta", "16.4", 1},
	} {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestApplyCatalogueOptions(t *testing.T) {
	versions := []map[string]any{
		{"id": "a", "name": "Xcode 16.2", "version": "16.2"},
		{"id": "b", "name": "Xcode 16.10", "version": "16.10"},
		{"id": "c", "name": "Xcode 15.4", "version": "15.4"},
	}
	repositories := []map[string]any{
		{"id": "r1", "repositoryName": "ios-app"},
		{"id": "r2", "repositoryName": "web"},
	}
	ids := func(records []map[string]any) string {
		var out []string
		for _, record := range records {
			out = append(out, stringField(record, "id"))
		}
		return strings.Join(out, ",")
	}

	xcode := &catalogueDataSource{kind: "ci_xcode_versions"}
	for name, tc := range map[string]struct {
		options catalogueOptions
		want    string
	}{
		"Apple's order by default":    {catalogueOptions{}, "a,b,c"},
		"numeric version, descending": {catalogueOptions{sortBy: "version", descending: true}, "b,a,c"},
		"the newest version":          {catalogueOptions{sortBy: "version", descending: true, limit: 1}, "b"},
		"name sorts as text":          {catalogueOptions{sortBy: "name"}, "c,b,a"},
		"name pattern":                {catalogueOptions{namePattern: regexp.MustCompile(`^Xcode 16`)}, "a,b"},
	} {
		if got := ids(xcode.applyCatalogueOptions(versions, tc.options)); got != tc.want {
			t.Errorf("%s: got %s, want %s", name, got, tc.want)
		}
	}

	scm := &catalogueDataSource{kind: "scm_repositories"}
	if got := ids(scm.applyCatalogueOptions(repositories, catalogueOptions{namePattern: regexp.MustCompile("ios")})); got != "r1" {
		t.Errorf("repository name pattern: got %s, want r1", got)
	}
}

// TestCatalogueReadAppliesTheConfiguration checks the whole Read: the options
// are read from the configuration, echoed into state, and the counts report
// before and after.
func TestCatalogueReadAppliesTheConfiguration(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[
			{"id":"a","attributes":{"name":"Xcode 16.2","version":"16.2"}},
			{"id":"b","attributes":{"name":"Xcode 16.10","version":"16.10"}},
			{"id":"c","attributes":{"name":"Xcode 15.4","version":"15.4"}}
		],"links":{}}`)
	}))
	defer server.Close()

	d := &catalogueDataSource{kind: "ci_mac_os_versions", client: &apple.Client{HostURL: server.URL, HTTPClient: server.Client(), Token: "test"}}
	var schemaResponse datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)
	objectType, ok := schemaResponse.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatal("schema is not an object")
	}
	raw := map[string]tftypes.Value{}
	for name, attributeType := range objectType.AttributeTypes {
		raw[name] = tftypes.NewValue(attributeType, nil)
	}
	raw["sort_by"] = tftypes.NewValue(tftypes.String, "version")
	raw["sort_order"] = tftypes.NewValue(tftypes.String, "desc")
	raw["limit"] = tftypes.NewValue(tftypes.Number, 1)

	response := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(objectType, raw)}}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}

	var state types.Object
	if diags := response.State.Get(ctx, &state); diags.HasError() {
		t.Fatal(diags)
	}
	attributes := state.Attributes()
	versions, ok := attributes["versions"].(types.List)
	if !ok || len(versions.Elements()) != 1 {
		t.Fatalf("versions = %v, want one", attributes["versions"])
	}
	newest, ok := versions.Elements()[0].(types.Object)
	if !ok || objectString(newest, "id") != "b" {
		t.Errorf("returned %v, want b, the newest", versions)
	}
	if attributes["total_count"].String() != "3" || attributes["filtered_count"].String() != "1" {
		t.Errorf("total_count %v, filtered_count %v; want 3 and 1", attributes["total_count"], attributes["filtered_count"])
	}
	if attributes["sort_by"].String() != `"version"` {
		t.Errorf("sort_by = %v, want the configured value kept", attributes["sort_by"])
	}
}
