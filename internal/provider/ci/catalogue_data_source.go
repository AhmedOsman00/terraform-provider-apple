// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var _ datasource.DataSourceWithConfigure = &catalogueDataSource{}

type catalogueDataSource struct {
	client *apple.Client
	kind   string
}

func NewCiProductsDataSource() datasource.DataSource {
	return &catalogueDataSource{kind: "ci_products"}
}
func NewScmRepositoriesDataSource() datasource.DataSource {
	return &catalogueDataSource{kind: "scm_repositories"}
}
func NewCiXcodeVersionsDataSource() datasource.DataSource {
	return &catalogueDataSource{kind: "ci_xcode_versions"}
}
func NewCiMacOsVersionsDataSource() datasource.DataSource {
	return &catalogueDataSource{kind: "ci_mac_os_versions"}
}
func (d *catalogueDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.kind
}
func computedString(description string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, MarkdownDescription: description}
}
func (d *catalogueDataSource) collection() string {
	switch d.kind {
	case "ci_products":
		return "products"
	case "scm_repositories":
		return "repositories"
	default:
		return "versions"
	}
}
func (d *catalogueDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := map[string]schema.Attribute{"id": computedString("The Apple-generated ID.")}
	description := ""
	switch d.kind {
	case "ci_products":
		description = "Lists Xcode Cloud products. Create the product in Xcode first, then use its ID as `product_id` on `apple_ci_workflow`."
		attributes["name"] = computedString("Product name.")
		attributes["created_date"] = computedString("Creation date.")
		attributes["product_type"] = computedString("`APP` or `FRAMEWORK`.")
	case "scm_repositories":
		description = "Lists repositories connected to Xcode Cloud. Use the repository's ID as `repository_id` on `apple_ci_workflow`. The SCM connection must already be set up in Xcode or App Store Connect."
		attributes["repository_name"] = computedString("Repository name.")
		attributes["owner_name"] = computedString("Repository owner.")
		attributes["http_clone_url"] = computedString("HTTPS clone URL.")
		attributes["ssh_clone_url"] = computedString("SSH clone URL.")
		attributes["last_accessed_date"] = computedString("Last access date.")
	case "ci_xcode_versions", "ci_mac_os_versions":
		description = "Lists macOS versions available in Xcode Cloud. Use the ID as `mac_os_version_id` on `apple_ci_workflow`. Apple checks compatibility with the selected Xcode version when the workflow is saved."
		attributes["name"] = computedString("Version display name.")
		attributes["version"] = computedString("Version number.")
		if d.kind == "ci_xcode_versions" {
			description = "Lists Xcode versions available in Xcode Cloud, with their test devices and runtimes. Use the ID as `xcode_version_id` on `apple_ci_workflow` and the destination identifiers in its test actions."
			attributes["test_destinations"] = schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Supported test devices and runtimes.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"device_type_name": computedString("Device display name."), "device_type_identifier": computedString("Device type identifier."), "kind": computedString("`SIMULATOR` or `MAC`."),
				"available_runtimes": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Available runtimes for this device.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"runtime_name": computedString("Runtime display name."), "runtime_identifier": computedString("Runtime identifier.")}}},
			}}}
		}
	}
	resp.Schema = schema.Schema{MarkdownDescription: description, Attributes: map[string]schema.Attribute{
		d.collection(): schema.ListNestedAttribute{Computed: true, MarkdownDescription: "The records returned by Apple.", NestedObject: schema.NestedAttributeObject{Attributes: attributes}},
	}}
}
func (d *catalogueDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apple.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *apple.Client, got %T", req.ProviderData))
		return
	}
	d.client = client
}
func (d *catalogueDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var records any
	var err error
	switch d.kind {
	case "ci_products":
		records, err = d.client.GetCiProducts()
	case "scm_repositories":
		records, err = d.client.GetScmRepositories()
	case "ci_xcode_versions":
		records, err = d.client.GetCiXcodeVersions()
	case "ci_mac_os_versions":
		records, err = d.client.GetCiMacOsVersions()
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Xcode Cloud Catalogue", err.Error())
		return
	}
	body, err := json.Marshal(records)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Encode Catalogue", err.Error())
		return
	}
	var decoded []struct {
		ID         string         `json:"id"`
		Attributes map[string]any `json:"attributes"`
	}
	if err = json.Unmarshal(body, &decoded); err != nil {
		resp.Diagnostics.AddError("Unable to Decode Catalogue", err.Error())
		return
	}
	values := make([]any, 0, len(decoded))
	for _, record := range decoded {
		value := record.Attributes
		if value == nil {
			value = map[string]any{}
		}
		value["id"] = record.ID
		values = append(values, value)
	}
	var schemaResponse datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)
	value, err := fromJSON(ctx, schemaResponse.Schema.Type(), map[string]any{d.collection(): values})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Record Catalogue", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, value)...)
}
