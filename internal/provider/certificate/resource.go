// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
	_ resource.ResourceWithModifyPlan  = &certificateResource{}
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
			"early_renewal_hours": schema.Int64Attribute{
				MarkdownDescription: "Replace the certificate this many hours before it expires.\n\n" +
					"Apple certificates are typically valid for one year, and builds break the moment one lapses. " +
					"Setting this to `720` (30 days) replaces the certificate a month early during a routine `terraform apply`, " +
					"rather than at the moment of expiry.\n\n" +
					"The window is evaluated at plan time against `expiration_date`, so renewal only happens when Terraform runs — " +
					"schedule a periodic plan/apply if you rely on it. Leave unset (or `0`) to disable early renewal.\n\n" +
					"~> **Note:** This must be smaller than the certificate's total validity period. A larger value puts the " +
					"certificate permanently inside its renewal window and replaces it on every apply.",
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"ready_for_renewal": schema.BoolAttribute{
				MarkdownDescription: "Whether the certificate has entered the window defined by `early_renewal_hours`. " +
					"This is a plan-time signal: it is recorded as `false` between runs, and flipping to `true` during a plan " +
					"is what forces the certificate to be replaced.",
				Computed: true,
			},
		},
	}
}

// optionalString maps an attribute Apple may omit to null rather than to an
// empty string, so an absent value is not mistaken for a value of "".
func optionalString(v string) types.String {
	if v == "" {
		return types.StringNull()
	}

	return types.StringValue(v)
}

// Create a new resource.
func (r *certificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating Certificate resource")

	// Retrieve values from plan
	var plan certificateResourceModel
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
	// Apple omits the requester fields on most responses; recording them as
	// null rather than "" keeps Create and Read agreeing on what absent means.
	plan.RequesterFirstName = optionalString(certificate.Attributes.RequesterFirstName)
	plan.RequesterLastName = optionalString(certificate.Attributes.RequesterLastName)
	plan.RequesterEmail = optionalString(certificate.Attributes.RequesterEmail)

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

	// A freshly issued certificate is never awaiting renewal.
	plan.ReadyForRenewal = types.BoolValue(false)

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
	var state certificateResourceModel
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
	state.CertificateType = types.StringValue(string(certificate.Attributes.CertificateType))

	// csr_content and the requester fields are deliberately not refreshed from
	// the response. Apple answers csrContent as null on every read and omits
	// requesterFirstName/LastName/Email entirely, so assigning them here writes
	// empty strings over values only the configuration and the create response
	// ever held. For csr_content that is destructive rather than cosmetic: the
	// attribute is RequiresReplace, so a blanked CSR makes the next plan revoke
	// the certificate and issue a new one. Prior state is the better record.
	if csr := certificate.Attributes.CsrContent; csr != "" {
		state.CsrContent = types.StringValue(csr)
	}

	if v := certificate.Attributes.RequesterFirstName; v != "" {
		state.RequesterFirstName = types.StringValue(v)
	}

	if v := certificate.Attributes.RequesterLastName; v != "" {
		state.RequesterLastName = types.StringValue(v)
	}

	if v := certificate.Attributes.RequesterEmail; v != "" {
		state.RequesterEmail = types.StringValue(v)
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

	// ready_for_renewal is a plan-time signal owned by ModifyPlan, which
	// compares the refreshed expiry against the configured window. Recording
	// false here is what lets the flip to true register as a change.
	state.ReadyForRenewal = types.BoolValue(false)

	tflog.Debug(ctx, "Certificate read successfully")

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
//
// The certificate itself is immutable and every attribute describing one
// requires replacement, so the only change that reaches this method is to
// early_renewal_hours: local bookkeeping that Apple does not store.
func (r *certificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan certificateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state certificateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the issued certificate forward untouched; only the window moves.
	plan.ID = state.ID
	plan.SerialNumber = state.SerialNumber
	plan.CertificateContent = state.CertificateContent
	plan.DisplayName = state.DisplayName
	plan.Name = state.Name
	plan.CsrContent = state.CsrContent
	plan.Platform = state.Platform
	plan.ExpirationDate = state.ExpirationDate
	plan.CertificateType = state.CertificateType
	plan.RequesterFirstName = state.RequesterFirstName
	plan.RequesterLastName = state.RequesterLastName
	plan.RequesterEmail = state.RequesterEmail
	plan.ReadyForRenewal = types.BoolValue(false)

	tflog.Info(ctx, "Updating Certificate early renewal window", map[string]interface{}{
		"certificate_id":      state.ID.ValueString(),
		"early_renewal_hours": plan.EarlyRenewalHours.ValueInt64(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *certificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting Certificate resource")

	// Retrieve values from state
	var state certificateResourceModel
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
	state := certificateResourceModel{
		ID:                 types.StringValue(certificate.ID),
		SerialNumber:       types.StringValue(certificate.Attributes.SerialNumber),
		CertificateContent: types.StringValue(certificate.Attributes.CertificateContent),
		DisplayName:        types.StringValue(certificate.Attributes.DisplayName),
		Name:               types.StringValue(certificate.Attributes.Name),
		CsrContent:         optionalString(certificate.Attributes.CsrContent),
		CertificateType:    types.StringValue(string(certificate.Attributes.CertificateType)),
		RequesterFirstName: optionalString(certificate.Attributes.RequesterFirstName),
		RequesterLastName:  optionalString(certificate.Attributes.RequesterLastName),
		RequesterEmail:     optionalString(certificate.Attributes.RequesterEmail),
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

	// Early renewal is configuration, which import cannot recover; it takes
	// effect on the next plan once the user declares it.
	state.EarlyRenewalHours = types.Int64Null()
	state.ReadyForRenewal = types.BoolValue(false)

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

// ModifyPlan replaces the certificate once it enters its early renewal window.
//
// Certificates are immutable, so renewal is a replacement: the plan marks every
// Apple-computed attribute unknown and reports ready_for_renewal as the
// attribute forcing it, which is what surfaces in `terraform plan` output.
func (r *certificateResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// No prior state on create, and no plan on destroy.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state certificateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan certificateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The renewal window is evaluated against the expiry Apple recorded, which
	// only exists in prior state.
	renew, err := readyForRenewal(state.ExpirationDate, plan.EarlyRenewalHours, time.Now())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Evaluate Certificate Renewal Window", err.Error())
		return
	}

	if !renew {
		plan.ReadyForRenewal = types.BoolValue(false)
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
		return
	}

	// Inside the window: everything Apple issues is recomputed from the
	// replacement certificate.
	plan.ReadyForRenewal = types.BoolValue(true)
	plan.ID = types.StringUnknown()
	plan.SerialNumber = types.StringUnknown()
	plan.CertificateContent = types.StringUnknown()
	plan.DisplayName = types.StringUnknown()
	plan.Name = types.StringUnknown()
	plan.Platform = types.StringUnknown()
	plan.ExpirationDate = types.StringUnknown()
	plan.RequesterFirstName = types.StringUnknown()
	plan.RequesterLastName = types.StringUnknown()
	plan.RequesterEmail = types.StringUnknown()

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.RequiresReplace = append(resp.RequiresReplace, path.Root("ready_for_renewal"))

	tflog.Info(ctx, "Certificate is within its early renewal window and will be replaced", map[string]interface{}{
		"certificate_id":      state.ID.ValueString(),
		"serial_number":       state.SerialNumber.ValueString(),
		"expiration_date":     state.ExpirationDate.ValueString(),
		"early_renewal_hours": plan.EarlyRenewalHours.ValueInt64(),
	})
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
