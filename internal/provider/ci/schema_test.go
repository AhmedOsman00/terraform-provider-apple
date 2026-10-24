// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestSchemas(t *testing.T) {
	ctx := context.Background()
	r := NewCiWorkflowResource()
	var rm resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "apple"}, &rm)
	if rm.TypeName != "apple_ci_workflow" {
		t.Fatal(rm.TypeName)
	}
	var rs resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &rs)
	if d := rs.Schema.ValidateImplementation(ctx); d.HasError() {
		t.Fatal(d)
	}
	for name, newDataSource := range map[string]func() datasource.DataSource{
		"apple_ci_products": NewCiProductsDataSource, "apple_scm_repositories": NewScmRepositoriesDataSource, "apple_ci_xcode_versions": NewCiXcodeVersionsDataSource, "apple_ci_mac_os_versions": NewCiMacOsVersionsDataSource,
	} {
		t.Run(name, func(t *testing.T) {
			d := newDataSource()
			var m datasource.MetadataResponse
			d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "apple"}, &m)
			if m.TypeName != name {
				t.Fatal(m.TypeName)
			}
			var s datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &s)
			if diag := s.Schema.ValidateImplementation(ctx); diag.HasError() {
				t.Fatal(diag)
			}
		})
	}
}
