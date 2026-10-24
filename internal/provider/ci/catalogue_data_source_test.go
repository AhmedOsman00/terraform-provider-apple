// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCatalogueRead(t *testing.T) {
	cases := []struct{ kind, path, attributes string }{
		{"ci_products", "ciProducts", `{"name":"App","productType":"APP","createdDate":"2026-01-01T00:00:00Z"}`},
		{"scm_repositories", "scmRepositories", `{"ownerName":"owner","repositoryName":"App","httpCloneUrl":"https://example.com/App.git","sshCloneUrl":"git@example.com:App.git"}`},
		{"ci_xcode_versions", "ciXcodeVersions", `{"name":"Xcode","version":"26","testDestinations":[{"deviceTypeName":"iPhone","deviceTypeIdentifier":"iphone","kind":"SIMULATOR","availableRuntimes":[{"runtimeName":"iOS","runtimeIdentifier":"default"}]}]}`},
		{"ci_mac_os_versions", "ciMacOsVersions", `{"name":"macOS","version":"26"}`},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			ctx := context.Background()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/"+tc.path {
					t.Error(r.URL.Path)
				}
				if _, err := fmt.Fprintf(w, `{"data":[{"id":"record","attributes":%s}],"links":{}}`, tc.attributes); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			d := &catalogueDataSource{kind: tc.kind, client: &apple.Client{HostURL: server.URL, HTTPClient: server.Client(), Token: "test"}}
			var schemaResponse datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)
			response := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			d.Read(ctx, datasource.ReadRequest{}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
			var state types.Object
			if diag := response.State.Get(ctx, &state); diag.HasError() {
				t.Fatal(diag)
			}
			records, ok := state.Attributes()[d.collection()].(types.List)
			if !ok || len(records.Elements()) != 1 {
				t.Fatal(state)
			}
			record, ok := records.Elements()[0].(types.Object)
			if !ok {
				t.Fatal("expected object")
			}
			if objectString(record, "id") != "record" {
				t.Fatal(record)
			}
			if tc.kind == "scm_repositories" && objectString(record, "http_clone_url") != "https://example.com/App.git" {
				t.Fatal(record)
			}
			if tc.kind == "ci_xcode_versions" {
				destinations, ok := record.Attributes()["test_destinations"].(types.List)
				if !ok || len(destinations.Elements()) != 1 {
					t.Fatal(record)
				}
				destination, ok := destinations.Elements()[0].(types.Object)
				if !ok || objectString(destination, "device_type_identifier") != "iphone" {
					t.Fatal(destinations)
				}
				runtimes, ok := destination.Attributes()["available_runtimes"].(types.List)
				if !ok || len(runtimes.Elements()) != 1 {
					t.Fatal(destination)
				}
				runtime, ok := runtimes.Elements()[0].(types.Object)
				if !ok || objectString(runtime, "runtime_identifier") != "default" {
					t.Fatal(runtimes)
				}
			}
		})
	}
}
func TestCatalogueReadEmptyAndError(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			ctx := context.Background()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				if _, err := fmt.Fprint(w, `{"data":[],"links":{}}`); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			d := &catalogueDataSource{kind: "ci_products", client: &apple.Client{HostURL: server.URL, HTTPClient: server.Client(), Token: "test"}}
			var schemaResponse datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)
			response := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			d.Read(ctx, datasource.ReadRequest{}, &response)
			if status != http.StatusOK {
				if !response.Diagnostics.HasError() {
					t.Fatal("API error was ignored")
				}
				return
			}
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
			var state types.Object
			if diag := response.State.Get(ctx, &state); diag.HasError() {
				t.Fatal(diag)
			}
			products, ok := state.Attributes()["products"].(types.List)
			if !ok || products.IsNull() || products.IsUnknown() || len(products.Elements()) != 0 {
				t.Fatal(state)
			}
		})
	}
}
