// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"fmt"
	"terraform-provider-apple/internal/apple"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &certificatesDataSource{}
	_ datasource.DataSourceWithConfigure = &certificatesDataSource{}
)

func NewCertificatesDataSource() datasource.DataSource {
	return &certificatesDataSource{}
}

// certificatesDataSource is the data source implementation.
type certificatesDataSource struct {
	client *apple.Client
}

// Metadata returns the data source type name.
func (d *certificatesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificates"
}

// Schema defines the schema for the data source.
func (d *certificatesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about Certificates from Apple App Store Connect.\n\n" +
			"This data source allows you to query and filter Certificates based on various criteria such as " +
			"certificate type, platform, name patterns, and serial numbers. It supports sorting and limiting results.",

		Attributes: map[string]schema.Attribute{
			// Filter attributes
			"certificate_type": schema.StringAttribute{
				MarkdownDescription: "Filter Certificates by type. Valid values include:\n" +
					"- `IOS_DEVELOPMENT` - iOS Development certificate\n" +
					"- `IOS_DISTRIBUTION` - iOS Distribution certificate\n" +
					"- `MAC_APP_DEVELOPMENT` - Mac App Development certificate\n" +
					"- `MAC_APP_DISTRIBUTION` - Mac App Distribution certificate\n" +
					"- `DEVELOPER_ID_APPLICATION` - Developer ID Application certificate\n" +
					"- `DEVELOPER_ID_INSTALLER` - Developer ID Installer certificate\n" +
					"- And others...\n\n" +
					"Cannot be used together with `certificate_types`.",
				Optional:   true,
				Validators: []validator.String{CertificateTypeValidator},
			},
			"certificate_types": schema.ListAttribute{
				MarkdownDescription: "Filter Certificates by multiple types. Accepts a list of certificate type values.\n" +
					"Cannot be used together with `certificate_type`.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  []validator.List{
					// TODO: Add custom validator to ensure mutual exclusivity with certificate_type
				},
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Filter Certificates by platform. Valid values are:\n" +
					"- `IOS` - iOS platform\n" +
					"- `MAC_OS` - macOS platform\n" +
					"- `TV_OS` - tvOS platform\n" +
					"- `WATCH_OS` - watchOS platform\n\n" +
					"Cannot be used together with `platforms`.",
				Optional:   true,
				Validators: []validator.String{PlatformValidator},
			},
			"platforms": schema.ListAttribute{
				MarkdownDescription: "Filter Certificates by multiple platforms. Accepts a list of platform values.\n" +
					"Cannot be used together with `platform`.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  []validator.List{
					// TODO: Add custom validator to ensure mutual exclusivity with platform
				},
			},
			"name_pattern": schema.StringAttribute{
				MarkdownDescription: "Filter Certificates by name using a glob pattern. " +
					"For example, `*Production*` to match Certificates containing 'Production' in the name.",
				Optional:   true,
				Validators: GetPatternValidator(),
			},
			"serial_number": schema.StringAttribute{
				MarkdownDescription: "Filter Certificates by exact serial number match.",
				Optional:            true,
				Validators:          GetSerialNumberValidator(),
			},

			// Result control attributes
			"limit": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of Certificates to return. Must be between 1 and 200.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 200),
				},
			},
			"sort_by": schema.StringAttribute{
				MarkdownDescription: "Field to sort Certificates by. Valid values are:\n" +
					"- `display_name` - Sort by Certificate display name\n" +
					"- `name` - Sort by Certificate name\n" +
					"- `serial_number` - Sort by serial number\n" +
					"- `certificate_type` - Sort by certificate type\n" +
					"- `expiration_date` - Sort by expiration date\n\n" +
					"Defaults to `display_name` if not specified.",
				Optional:   true,
				Validators: []validator.String{SortByValidator},
			},
			"sort_order": schema.StringAttribute{
				MarkdownDescription: "Sort order for the results. Valid values are:\n" +
					"- `asc` - Ascending order (default)\n" +
					"- `desc` - Descending order",
				Optional:   true,
				Validators: []validator.String{SortOrderValidator},
			},

			// Output attributes
			"certificates": schema.ListNestedAttribute{
				MarkdownDescription: "List of Certificates that match the specified filters.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique Apple-generated identifier for the Certificate.",
							Computed:            true,
						},
						"serial_number": schema.StringAttribute{
							MarkdownDescription: "The serial number of the certificate.",
							Computed:            true,
						},
						"certificate_content": schema.StringAttribute{
							MarkdownDescription: "The Base64-encoded certificate content.",
							Computed:            true,
							Sensitive:           true,
						},
						"display_name": schema.StringAttribute{
							MarkdownDescription: "The display name for the Certificate.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the Certificate.",
							Computed:            true,
						},
						"csr_content": schema.StringAttribute{
							MarkdownDescription: "The Certificate Signing Request (CSR) content used to create this certificate.",
							Computed:            true,
						},
						"platform": schema.StringAttribute{
							MarkdownDescription: "The platform for the Certificate (IOS, MAC_OS, TV_OS, WATCH_OS).",
							Computed:            true,
						},
						"expiration_date": schema.StringAttribute{
							MarkdownDescription: "The expiration date of the certificate in RFC3339 format.",
							Computed:            true,
						},
						"certificate_type": schema.StringAttribute{
							MarkdownDescription: "The type of the certificate.",
							Computed:            true,
						},
						"requester_first_name": schema.StringAttribute{
							MarkdownDescription: "The first name of the person who requested the certificate.",
							Computed:            true,
						},
						"requester_last_name": schema.StringAttribute{
							MarkdownDescription: "The last name of the person who requested the certificate.",
							Computed:            true,
						},
						"requester_email": schema.StringAttribute{
							MarkdownDescription: "The email of the person who requested the certificate.",
							Computed:            true,
						},
					},
				},
			},

			// Computed metadata attributes
			"total_count": schema.Int64Attribute{
				MarkdownDescription: "Total number of Certificates retrieved from Apple before filtering.",
				Computed:            true,
			},
			"filtered_count": schema.Int64Attribute{
				MarkdownDescription: "Number of Certificates after applying filters and before applying limit.",
				Computed:            true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *certificatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Info(ctx, "Reading Certificates data source")

	var config certificatesDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Fetching Certificates from Apple API")

	// Get all Certificates from Apple
	certificates, err := d.client.GetCertificates()
	if err != nil {
		tflog.Error(ctx, "Failed to fetch Certificates", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Unable to Read Certificates",
			fmt.Sprintf("Could not retrieve Certificates from Apple App Store Connect: %s", err.Error()),
		)
		return
	}

	totalCount := len(certificates)
	tflog.Debug(ctx, "Retrieved Certificates from Apple", map[string]interface{}{
		"total_count": totalCount,
	})

	// Parse filter options from configuration
	filterOptions, err := ParseFilterOptions(ctx, config)
	if err != nil {
		tflog.Error(ctx, "Failed to parse filter options", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Invalid Filter Configuration",
			fmt.Sprintf("Could not parse filter options: %s", err.Error()),
		)
		return
	}

	// Apply filters
	filteredCertificates, err := FilterCertificates(ctx, certificates, filterOptions)
	if err != nil {
		tflog.Error(ctx, "Failed to filter Certificates", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Filter Error",
			fmt.Sprintf("Could not apply filters to Certificates: %s", err.Error()),
		)
		return
	}

	filteredCount := len(filteredCertificates)
	tflog.Info(ctx, "Applied filters to Certificates", map[string]interface{}{
		"total_count":    totalCount,
		"filtered_count": filteredCount,
		"final_count":    len(filteredCertificates), // After limit
	})

	// Map Certificates to model
	state := certificatesDataSourceModel{
		Certificates:  make([]certificateModel, 0, len(filteredCertificates)),
		TotalCount:    types.Int64Value(int64(totalCount)),
		FilteredCount: types.Int64Value(int64(filteredCount)),
	}

	// Copy filter configuration to state for reference
	state.CertificateType = config.CertificateType
	state.CertificateTypes = config.CertificateTypes
	state.Platform = config.Platform
	state.Platforms = config.Platforms
	state.NamePattern = config.NamePattern
	state.SerialNumber = config.SerialNumber
	state.Limit = config.Limit
	state.SortBy = config.SortBy
	state.SortOrder = config.SortOrder

	for _, certificate := range filteredCertificates {
		certModel := certificateModel{
			ID:                 types.StringValue(certificate.ID),
			SerialNumber:       types.StringValue(certificate.Attributes.SerialNumber),
			CertificateContent: types.StringValue(certificate.Attributes.CertificateContent),
			DisplayName:        types.StringValue(certificate.Attributes.DisplayName),
			Name:               types.StringValue(certificate.Attributes.Name),
			CsrContent:         types.StringValue(certificate.Attributes.CsrContent),
			CertificateType:    types.StringValue(string(certificate.Attributes.CertificateType)),
			RequesterFirstName: types.StringValue(certificate.Attributes.RequesterFirstName),
			RequesterLastName:  types.StringValue(certificate.Attributes.RequesterLastName),
			RequesterEmail:     types.StringValue(certificate.Attributes.RequesterEmail),
		}

		// Handle optional platform field
		if certificate.Attributes.Platform != nil {
			certModel.Platform = types.StringValue(string(*certificate.Attributes.Platform))
		} else {
			certModel.Platform = types.StringNull()
		}

		// Handle optional expiration date field
		if certificate.Attributes.ExpirationDate != nil {
			certModel.ExpirationDate = types.StringValue(certificate.Attributes.ExpirationDate.Format(time.RFC3339))
		} else {
			certModel.ExpirationDate = types.StringNull()
		}

		state.Certificates = append(state.Certificates, certModel)
	}

	tflog.Debug(ctx, "Certificates data source read successfully", map[string]interface{}{
		"result_count": len(state.Certificates),
	})

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Configure adds the provider configured client to the data source.
func (d *certificatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apple.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *apple.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}
