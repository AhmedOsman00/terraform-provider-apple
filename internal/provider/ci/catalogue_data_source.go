// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	attributes := map[string]schema.Attribute{"id": computedString("Apple's ID for the record, which `apple_ci_workflow` takes as a relationship.")}
	description := ""
	nameAttribute := "name"
	switch d.kind {
	case "ci_products":
		description = "Lists Xcode Cloud products. Create the product in Xcode first, then use its ID as `product_id` on `apple_ci_workflow`."
		attributes["name"] = computedString("The product's name, as Xcode Cloud shows it: normally the app or framework target.")
		attributes["created_date"] = computedString("When the product was created in Xcode Cloud, in ISO 8601.")
		attributes["product_type"] = computedString("What the product builds: `APP` or `FRAMEWORK`.")
	case "scm_repositories":
		nameAttribute = "repository_name"
		description = "Lists repositories connected to Xcode Cloud. Use the repository's ID as `repository_id` on `apple_ci_workflow`. The SCM connection must already be set up in Xcode or App Store Connect."
		attributes["repository_name"] = computedString("The repository's name at the source control provider, without the owner.")
		attributes["owner_name"] = computedString("The account or organisation that owns the repository.")
		attributes["http_clone_url"] = computedString("The repository's HTTPS clone URL.")
		attributes["ssh_clone_url"] = computedString("The repository's SSH clone URL.")
		attributes["last_accessed_date"] = computedString("When Xcode Cloud last read the repository, in ISO 8601.")
	case "ci_xcode_versions", "ci_mac_os_versions":
		description = "Lists macOS versions available in Xcode Cloud. Use the ID as `mac_os_version_id` on `apple_ci_workflow`. Apple checks compatibility with the selected Xcode version when the workflow is saved."
		attributes["name"] = computedString("The version's display name, as Xcode Cloud lists it.")
		attributes["version"] = computedString("The version number alone, without the name. `sort_by = \"version\"` orders by it numerically.")
		if d.kind == "ci_xcode_versions" {
			description = "Lists Xcode versions available in Xcode Cloud, with their test devices and runtimes. Use the ID as `xcode_version_id` on `apple_ci_workflow` and the destination identifiers in its test actions."
			attributes["name"] = computedString("The version's display name, as Xcode Cloud lists it.")
			attributes["version"] = computedString("The version number alone, without the name. `sort_by = \"version\"` orders by it numerically.")
			attributes["test_destinations"] = schema.ListNestedAttribute{Computed: true, MarkdownDescription: "The devices this Xcode version can run tests on, with the runtimes each supports.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"device_type_name":       computedString("The device's display name, as Xcode Cloud lists it."),
				"device_type_identifier": computedString("The identifier `apple_ci_workflow` takes as a test destination's `device_type_identifier`."),
				"kind":                   computedString("Where the tests run: `SIMULATOR` or `MAC`."),
				"available_runtimes": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "The runtimes this device can run tests on.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"runtime_name":       computedString("The runtime's display name, as Xcode Cloud lists it."),
					"runtime_identifier": computedString("The identifier `apple_ci_workflow` takes as a test destination's `runtime_identifier`."),
				}}},
			}}}
		}
	}
	resp.Schema = schema.Schema{MarkdownDescription: description, Attributes: map[string]schema.Attribute{
		d.collection(): schema.ListNestedAttribute{Computed: true, MarkdownDescription: "The records that match, in the order asked for.", NestedObject: schema.NestedAttributeObject{Attributes: attributes}},
		"name_pattern": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Return only records whose `" + nameAttribute + "` matches this regular expression, matched as a substring unless anchored.",
		},
		"sort_by": schema.StringAttribute{
			Optional: true,
			MarkdownDescription: "The attribute to sort by: `" + strings.Join(d.sortKeys(), "`, `") + "`. Unset keeps Apple's order." +
				versionSortNote(d.kind),
			Validators: []validator.String{stringvalidator.OneOf(d.sortKeys()...)},
		},
		"sort_order": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "`asc` (the default) or `desc`. Has no effect without `sort_by`.",
			Validators:          []validator.String{stringvalidator.OneOf("asc", "desc")},
		},
		"limit": schema.Int64Attribute{
			Optional:            true,
			MarkdownDescription: "The most records to return, between 1 and 200, after filtering and sorting.",
			Validators:          []validator.Int64{int64validator.Between(1, 200)},
		},
		"total_count":    schema.Int64Attribute{Computed: true, MarkdownDescription: "How many records Apple returned, before filtering."},
		"filtered_count": schema.Int64Attribute{Computed: true, MarkdownDescription: "How many records are returned, after filtering and the limit."},
	}}
}

// versionSortNote explains the one sort that is not alphabetical.
func versionSortNote(kind string) string {
	if kind == "ci_xcode_versions" || kind == "ci_mac_os_versions" {
		return " `version` compares numerically, so `16.10` sorts after `16.2`: with `sort_order = \"desc\"` and `limit = 1` it returns the newest version."
	}
	return ""
}

// catalogueConfig reads what the configuration asked for. A request with no
// configuration, as in a test, asks for nothing.
func (d *catalogueDataSource) catalogueConfig(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) (catalogueOptions, map[string]any) {
	options := catalogueOptions{}
	echoed := map[string]any{}
	if config.Raw.IsNull() {
		return options, echoed
	}

	var pattern, sortBy, sortOrder types.String
	var limit types.Int64
	diags.Append(config.GetAttribute(ctx, path.Root("name_pattern"), &pattern)...)
	diags.Append(config.GetAttribute(ctx, path.Root("sort_by"), &sortBy)...)
	diags.Append(config.GetAttribute(ctx, path.Root("sort_order"), &sortOrder)...)
	diags.Append(config.GetAttribute(ctx, path.Root("limit"), &limit)...)
	if diags.HasError() {
		return options, echoed
	}

	if !pattern.IsNull() && !pattern.IsUnknown() {
		compiled, err := regexp.Compile(pattern.ValueString())
		if err != nil {
			diags.AddAttributeError(path.Root("name_pattern"), "Invalid Regular Expression", err.Error())
			return options, echoed
		}
		options.namePattern = compiled
		echoed["name_pattern"] = pattern.ValueString()
	}
	if !sortBy.IsNull() {
		options.sortBy = sortBy.ValueString()
		echoed["sort_by"] = options.sortBy
	}
	if !sortOrder.IsNull() {
		options.descending = sortOrder.ValueString() == "desc"
		echoed["sort_order"] = sortOrder.ValueString()
	}
	if !limit.IsNull() {
		options.limit = int(limit.ValueInt64())
		echoed["limit"] = float64(limit.ValueInt64())
	}
	return options, echoed
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
func (d *catalogueDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	options, echoed := d.catalogueConfig(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

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
	all := make([]map[string]any, 0, len(decoded))
	for _, record := range decoded {
		value := record.Attributes
		if value == nil {
			value = map[string]any{}
		}
		value["id"] = record.ID
		all = append(all, value)
	}
	kept := d.applyCatalogueOptions(all, options)
	values := make([]any, 0, len(kept))
	for _, record := range kept {
		values = append(values, record)
	}

	echoed[d.collection()] = values
	echoed["total_count"] = float64(len(all))
	echoed["filtered_count"] = float64(len(kept))

	var schemaResponse datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)
	value, err := fromJSON(ctx, schemaResponse.Schema.Type(), echoed)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Record Catalogue", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, value)...)
}
