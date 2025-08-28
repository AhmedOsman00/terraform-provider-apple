package certificate

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-apple/internal/apple"
	"terraform-provider-apple/internal/apple/models"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &certificateResource{}
	_ resource.ResourceWithConfigure   = &certificateResource{}
	_ resource.ResourceWithImportState = &certificateResource{}
)

func NewCertificateResource() resource.Resource {
	return &certificateResource{}
}

type certificateResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *certificateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

// Schema defines the schema for the resource.
func (r *certificateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Apple App Store Connect Certificate.\n\n" +
			"Certificates are used for code signing, app distribution, and other Apple development purposes. " +
			"Once created, certificates cannot be modified - they can only be created or revoked.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the Certificate. Used for API references.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"certificate_type": schema.StringAttribute{
				MarkdownDescription: "The type of certificate to create. Valid values include:\n" +
					"- `IOS_DEVELOPMENT` - iOS Development certificate\n" +
					"- `IOS_DISTRIBUTION` - iOS Distribution certificate\n" +
					"- `MAC_APP_DEVELOPMENT` - Mac App Development certificate\n" +
					"- `MAC_APP_DISTRIBUTION` - Mac App Distribution certificate\n" +
					"- `DEVELOPER_ID_APPLICATION` - Developer ID Application certificate\n" +
					"- `DEVELOPER_ID_INSTALLER` - Developer ID Installer certificate\n" +
					"- And others...\n\n" +
					"This cannot be changed after creation.",
				Required:   true,
				Validators: []validator.String{CertificateTypeValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"csr_content": schema.StringAttribute{
				MarkdownDescription: "The Certificate Signing Request (CSR) content in PEM format. " +
					"This is used to generate the certificate and cannot be changed after creation.",
				Required:   true,
				Validators: GetCsrContentValidator(),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"serial_number": schema.StringAttribute{
				MarkdownDescription: "The serial number of the certificate, automatically assigned by Apple.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"certificate_content": schema.StringAttribute{
				MarkdownDescription: "The Base64-encoded certificate content. This contains the actual certificate data.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name for the Certificate, automatically assigned by Apple.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the Certificate, automatically assigned by Apple.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "The platform for the Certificate, if applicable.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"expiration_date": schema.StringAttribute{
				MarkdownDescription: "The expiration date of the certificate in RFC3339 format.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"requester_first_name": schema.StringAttribute{
				MarkdownDescription: "The first name of the person who requested the certificate.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"requester_last_name": schema.StringAttribute{
				MarkdownDescription: "The last name of the person who requested the certificate.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"requester_email": schema.StringAttribute{
				MarkdownDescription: "The email of the person who requested the certificate.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *certificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating Certificate resource")

	// Retrieve values from plan
	var plan certificateModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "certificate_type", plan.CertificateType.ValueString())

	tflog.Debug(ctx, "Creating Certificate with Apple API")

	// Create new Certificate
	certificate, err := r.client.CreateCertificate(
		models.CertificateType(plan.CertificateType.ValueString()),
		plan.CsrContent.ValueString(),
		nil,
	)
	if err != nil {
		// Provide specific error messages for common issues
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid csr") || strings.Contains(errMsg, "csr") {
			resp.Diagnostics.AddError(
				"Invalid Certificate Signing Request",
				fmt.Sprintf("The Certificate Signing Request (CSR) is invalid. Please ensure it's a valid PEM-encoded CSR: %s", err.Error()),
			)
		} else if strings.Contains(errMsg, "quota") || strings.Contains(errMsg, "limit") {
			resp.Diagnostics.AddError(
				"Certificate Quota Exceeded",
				fmt.Sprintf("You have reached the limit for certificates of this type. Please revoke unused certificates first: %s", err.Error()),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Creating Certificate",
				fmt.Sprintf("Could not create Certificate of type '%s': %s", plan.CertificateType.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create Certificate", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Map response body to schema and populate computed attribute values
	plan.ID = types.StringValue(certificate.ID)
	plan.SerialNumber = types.StringValue(certificate.Attributes.SerialNumber)
	plan.CertificateContent = types.StringValue(certificate.Attributes.CertificateContent)
	plan.DisplayName = types.StringValue(certificate.Attributes.DisplayName)
	plan.Name = types.StringValue(certificate.Attributes.Name)
	plan.RequesterFirstName = types.StringValue(certificate.Attributes.RequesterFirstName)
	plan.RequesterLastName = types.StringValue(certificate.Attributes.RequesterLastName)
	plan.RequesterEmail = types.StringValue(certificate.Attributes.RequesterEmail)

	// Handle optional platform field
	if certificate.Attributes.Platform != nil {
		plan.Platform = types.StringValue(string(*certificate.Attributes.Platform))
	} else {
		plan.Platform = types.StringNull()
	}

	// Handle optional expiration date field
	if certificate.Attributes.ExpirationDate != nil {
		plan.ExpirationDate = types.StringValue(certificate.Attributes.ExpirationDate.Format(time.RFC3339))
	} else {
		plan.ExpirationDate = types.StringNull()
	}

	tflog.Info(ctx, "Certificate created successfully", map[string]interface{}{
		"certificate_id":   certificate.ID,
		"serial_number":    certificate.Attributes.SerialNumber,
		"certificate_type": string(certificate.Attributes.CertificateType),
	})

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *certificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading Certificate resource")

	// Get current state
	var state certificateModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "certificate_id", state.ID.ValueString())

	// Get refreshed Certificate value from Apple
	certificate, err := r.client.GetCertificate(state.ID.ValueString())
	if err != nil {
		// Handle 404 - resource no longer exists
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, "Certificate not found, removing from state")
			resp.Diagnostics.AddWarning(
				"Certificate Not Found",
				fmt.Sprintf("Certificate %s was not found and will be removed from Terraform state. It may have been revoked outside of Terraform.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}

		tflog.Error(ctx, "Failed to read Certificate", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Reading Certificate",
			fmt.Sprintf("Could not read Certificate %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Overwrite Certificate with refreshed state
	state.ID = types.StringValue(certificate.ID)
	state.SerialNumber = types.StringValue(certificate.Attributes.SerialNumber)
	state.CertificateContent = types.StringValue(certificate.Attributes.CertificateContent)
	state.DisplayName = types.StringValue(certificate.Attributes.DisplayName)
	state.Name = types.StringValue(certificate.Attributes.Name)
	state.CsrContent = types.StringValue(certificate.Attributes.CsrContent)
	state.CertificateType = types.StringValue(string(certificate.Attributes.CertificateType))
	state.RequesterFirstName = types.StringValue(certificate.Attributes.RequesterFirstName)
	state.RequesterLastName = types.StringValue(certificate.Attributes.RequesterLastName)
	state.RequesterEmail = types.StringValue(certificate.Attributes.RequesterEmail)

	// Handle optional platform field
	if certificate.Attributes.Platform != nil {
		state.Platform = types.StringValue(string(*certificate.Attributes.Platform))
	} else {
		state.Platform = types.StringNull()
	}

	// Handle optional expiration date field
	if certificate.Attributes.ExpirationDate != nil {
		state.ExpirationDate = types.StringValue(certificate.Attributes.ExpirationDate.Format(time.RFC3339))
	} else {
		state.ExpirationDate = types.StringNull()
	}

	tflog.Debug(ctx, "Certificate read successfully")

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *certificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Certificates cannot be updated - they are immutable after creation
	// This should never be called due to RequiresReplace plan modifiers on all configurable attributes
	resp.Diagnostics.AddError(
		"Certificate Update Not Supported",
		"Certificates cannot be updated after creation. All certificate attributes require replacement when changed.",
	)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *certificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting Certificate resource")

	// Retrieve values from state
	var state certificateModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "certificate_id", state.ID.ValueString())
	ctx = tflog.SetField(ctx, "serial_number", state.SerialNumber.ValueString())

	tflog.Debug(ctx, "Revoking Certificate with Apple API")

	// Delete (revoke) Certificate
	err := r.client.DeleteCertificate(state.ID.ValueString(), nil)
	if err != nil {
		// Handle specific delete errors
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			// Resource already deleted, this is not an error
			tflog.Warn(ctx, "Certificate already revoked outside of Terraform")
			resp.Diagnostics.AddWarning(
				"Certificate Already Revoked",
				fmt.Sprintf("Certificate %s was already revoked outside of Terraform.", state.ID.ValueString()),
			)
			return
		}

		tflog.Error(ctx, "Failed to revoke Certificate", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Revoking Certificate",
			fmt.Sprintf("Could not revoke Certificate %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Certificate revoked successfully")
}

// ImportState imports the resource into Terraform state.
func (r *certificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing Certificate resource", map[string]interface{}{
		"import_id": req.ID,
	})

	// Import supports Certificate ID, serial number, or display name
	importID := strings.TrimSpace(req.ID)

	if importID == "" {
		resp.Diagnostics.AddError(
			"Empty Import Identifier",
			"The import identifier cannot be empty. Provide either the Certificate ID (Apple's internal ID), serial number, or display name.",
		)
		return
	}

	var certificate *models.Certificate
	var err error

	// Try to import by Apple ID first (if it looks like an Apple ID)
	if strings.HasPrefix(importID, "cert-") || len(importID) > 50 {
		tflog.Debug(ctx, "Attempting import by Apple Certificate ID")
		certificate, err = r.client.GetCertificate(importID)
	} else {
		// Try to import by serial number or name
		tflog.Debug(ctx, "Attempting import by serial number or name")

		// First try by serial number
		certificate, err = r.client.GetCertificateBySerialNumber(importID)
		if err != nil {
			// If that fails, try by name
			certificate, err = r.client.GetCertificateByName(importID)
		}
	}

	if err != nil {
		tflog.Error(ctx, "Failed to import Certificate", map[string]interface{}{
			"import_id": importID,
			"error":     err.Error(),
		})
		resp.Diagnostics.AddError(
			"Certificate Not Found",
			fmt.Sprintf("Could not find Certificate with identifier '%s'. Please verify the identifier exists in your Apple Developer account. Error: %s", importID, err.Error()),
		)
		return
	}

	// Populate the state with the found Certificate
	state := certificateModel{
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
		state.Platform = types.StringValue(string(*certificate.Attributes.Platform))
	} else {
		state.Platform = types.StringNull()
	}

	// Handle optional expiration date field
	if certificate.Attributes.ExpirationDate != nil {
		state.ExpirationDate = types.StringValue(certificate.Attributes.ExpirationDate.Format(time.RFC3339))
	} else {
		state.ExpirationDate = types.StringNull()
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Certificate imported successfully", map[string]interface{}{
		"certificate_id":   certificate.ID,
		"serial_number":    certificate.Attributes.SerialNumber,
		"certificate_type": string(certificate.Attributes.CertificateType),
	})

	resp.Diagnostics.AddWarning(
		"Certificate Imported Successfully",
		fmt.Sprintf("Certificate '%s' (serial: %s, type: %s) has been imported. Please review the configuration and run 'terraform plan' to see any differences.",
			certificate.ID, certificate.Attributes.SerialNumber, string(certificate.Attributes.CertificateType)),
	)
}

// Configure adds the provider configured client to the resource.
func (r *certificateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apple.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *apple.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}
