package profile

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-apple/internal/apple"
	"terraform-provider-apple/internal/apple/models"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &profileResource{}
	_ resource.ResourceWithConfigure   = &profileResource{}
	_ resource.ResourceWithImportState = &profileResource{}
)

func NewProfileResource() resource.Resource {
	return &profileResource{}
}

type profileResource struct {
	client *apple.Client
}

// platformValue converts Apple's computed platform into state, mapping an
// absent value to null rather than an empty string.
func platformValue(p models.ProfilePlatform) types.String {
	if p == "" {
		return types.StringNull()
	}
	return types.StringValue(string(p))
}

// Metadata returns the resource type name.
func (r *profileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile"
}

// Schema defines the schema for the resource.
func (r *profileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Apple App Store Connect Provisioning Profile.\n\n" +
			"Provisioning profiles are used to link Bundle IDs, certificates, and devices " +
			"for code signing and app distribution in the Apple ecosystem.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the Profile. Used for API references.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "A human-readable name for the Profile. This can be updated after creation.",
				Required:            true,
				Validators:          GetNameValidator(),
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "The platform the Profile targets (`IOS`, `MAC_OS`, `TV_OS`, or `WATCH_OS`).\n\n" +
					"This is derived by Apple from `profile_type` rather than set directly — " +
					"an `IOS_APP_STORE` profile reports `IOS`, a `MAC_APP_DEVELOPMENT` profile reports `MAC_OS`.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile_type": schema.StringAttribute{
				MarkdownDescription: "The type of Profile to create. This determines both the platform and the " +
					"distribution method, and is the attribute that decides whether the profile can be used for " +
					"development, ad hoc testing, or App Store submission. Valid values are:\n" +
					"- `IOS_APP_DEVELOPMENT` - iOS development, requires `devices`\n" +
					"- `IOS_APP_ADHOC` - iOS ad hoc distribution, requires `devices`\n" +
					"- `IOS_APP_STORE` - iOS App Store distribution\n" +
					"- `IOS_APP_INHOUSE` - iOS in-house (Enterprise) distribution\n" +
					"- `MAC_APP_DEVELOPMENT` - macOS development\n" +
					"- `MAC_APP_STORE` - macOS App Store distribution\n" +
					"- `MAC_APP_DIRECT` - macOS Developer ID distribution\n" +
					"- `TVOS_APP_DEVELOPMENT` - tvOS development, requires `devices`\n" +
					"- `TVOS_APP_ADHOC` - tvOS ad hoc distribution, requires `devices`\n" +
					"- `TVOS_APP_STORE` - tvOS App Store distribution\n" +
					"- `TVOS_APP_INHOUSE` - tvOS in-house (Enterprise) distribution\n\n" +
					"This cannot be changed after creation.",
				Required:   true,
				Validators: []validator.String{ProfileTypeValidator},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bundle_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the Bundle ID to associate with this profile. This cannot be changed after creation.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"certificates": schema.ListAttribute{
				MarkdownDescription: "List of certificate IDs to include in the profile. This cannot be changed after creation.",
				ElementType:         types.StringType,
				Required:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"devices": schema.ListAttribute{
				MarkdownDescription: "List of device IDs to include in the profile. Optional for App Store profiles. This cannot be changed after creation.",
				ElementType:         types.StringType,
				Optional:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"profile_content": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded `.mobileprovision` content. This is computed by Apple.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uuid": schema.StringAttribute{
				MarkdownDescription: "The UUID of the profile. This is computed by Apple.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile_state": schema.StringAttribute{
				MarkdownDescription: "The current state of the profile (ACTIVE, INVALID, or EXPIRED). This is computed by Apple.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_date": schema.StringAttribute{
				MarkdownDescription: "The date and time when the profile was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"expiration_date": schema.StringAttribute{
				MarkdownDescription: "The date and time when the profile expires.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create a new resource.
func (r *profileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating Profile resource")

	// Retrieve values from plan
	var plan profileModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add structured logging
	ctx = tflog.SetField(ctx, "profile_name", plan.Name.ValueString())
	ctx = tflog.SetField(ctx, "profile_type", plan.ProfileType.ValueString())
	ctx = tflog.SetField(ctx, "bundle_id", plan.BundleID.ValueString())

	tflog.Debug(ctx, "Creating Profile with Apple API")

	// Extract certificate IDs
	var certificateElements []types.String
	diags = plan.Certificates.ElementsAs(ctx, &certificateElements, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	certificateIDs := make([]string, len(certificateElements))
	for i, cert := range certificateElements {
		certificateIDs[i] = cert.ValueString()
	}

	// Extract device IDs (optional)
	var deviceIDs []string
	if !plan.Devices.IsNull() {
		var deviceElements []types.String
		diags = plan.Devices.ElementsAs(ctx, &deviceElements, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		deviceIDs = make([]string, len(deviceElements))
		for i, device := range deviceElements {
			deviceIDs[i] = device.ValueString()
		}
	}

	// Create new Profile
	profile, err := r.client.CreateProfile(
		plan.Name.ValueString(),
		models.ProfileType(plan.ProfileType.ValueString()),
		plan.BundleID.ValueString(),
		certificateIDs,
		deviceIDs,
		nil,
	)
	if err != nil {
		// Provide specific error messages for common issues
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			resp.Diagnostics.AddError(
				"Profile Already Exists",
				fmt.Sprintf("A Profile with name '%s' already exists in your Apple Developer account. Profile names must be unique.", plan.Name.ValueString()),
			)
		} else if strings.Contains(errMsg, "invalid bundle") {
			resp.Diagnostics.AddError(
				"Invalid Bundle ID",
				fmt.Sprintf("The Bundle ID '%s' is invalid or does not exist.", plan.BundleID.ValueString()),
			)
		} else if strings.Contains(errMsg, "invalid certificate") {
			resp.Diagnostics.AddError(
				"Invalid Certificate",
				"One or more of the specified certificates are invalid or do not exist.",
			)
		} else if strings.Contains(errMsg, "invalid device") {
			resp.Diagnostics.AddError(
				"Invalid Device",
				"One or more of the specified devices are invalid or do not exist.",
			)
		} else {
			resp.Diagnostics.AddError(
				"Error Creating Profile",
				fmt.Sprintf("Could not create Profile with name '%s': %s", plan.Name.ValueString(), err.Error()),
			)
		}
		tflog.Error(ctx, "Failed to create Profile", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Map response body to schema and populate computed attribute values.
	// Every computed attribute must be given an explicit value, including null:
	// one left unknown after apply fails with "Provider produced inconsistent
	// result after apply".
	plan.ID = types.StringValue(profile.ID)
	plan.Platform = platformValue(profile.Attributes.Platform)
	if profile.Attributes.ProfileContent != nil {
		plan.ProfileContent = types.StringValue(*profile.Attributes.ProfileContent)
	} else {
		plan.ProfileContent = types.StringNull()
	}
	if profile.Attributes.UUID != nil {
		plan.UUID = types.StringValue(*profile.Attributes.UUID)
	} else {
		plan.UUID = types.StringNull()
	}
	if profile.Attributes.ProfileState != nil {
		plan.ProfileState = types.StringValue(string(*profile.Attributes.ProfileState))
	} else {
		plan.ProfileState = types.StringNull()
	}
	// profile_type is configured, so leave the planned value in place when Apple
	// omits it: overwriting with null would contradict the config after apply.
	if profile.Attributes.ProfileType != nil {
		plan.ProfileType = types.StringValue(string(*profile.Attributes.ProfileType))
	}
	if profile.Attributes.CreatedDate != nil {
		plan.CreatedDate = types.StringValue(profile.Attributes.CreatedDate.Format(time.RFC3339))
	} else {
		plan.CreatedDate = types.StringNull()
	}
	if profile.Attributes.ExpirationDate != nil {
		plan.ExpirationDate = types.StringValue(profile.Attributes.ExpirationDate.Format(time.RFC3339))
	} else {
		plan.ExpirationDate = types.StringNull()
	}

	tflog.Info(ctx, "Profile created successfully", map[string]interface{}{
		"profile_id": profile.ID,
		"uuid":       profile.Attributes.UUID,
	})

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *profileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading Profile resource")

	// Get current state
	var state profileModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "profile_id", state.ID.ValueString())

	// Get refreshed Profile value from Apple
	profile, err := r.client.GetProfile(state.ID.ValueString())
	if err != nil {
		// Handle 404 - resource no longer exists
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, "Profile not found, removing from state")
			resp.Diagnostics.AddWarning(
				"Profile Not Found",
				fmt.Sprintf("Profile %s was not found and will be removed from Terraform state. It may have been deleted outside of Terraform.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}

		tflog.Error(ctx, "Failed to read Profile", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Reading Profile",
			fmt.Sprintf("Could not read Profile %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Overwrite Profile with refreshed state
	state.ID = types.StringValue(profile.ID)
	state.Name = types.StringValue(profile.Attributes.Name)
	state.Platform = platformValue(profile.Attributes.Platform)

	if profile.Attributes.ProfileContent != nil {
		state.ProfileContent = types.StringValue(*profile.Attributes.ProfileContent)
	} else {
		state.ProfileContent = types.StringNull()
	}
	if profile.Attributes.UUID != nil {
		state.UUID = types.StringValue(*profile.Attributes.UUID)
	} else {
		state.UUID = types.StringNull()
	}
	if profile.Attributes.ProfileState != nil {
		state.ProfileState = types.StringValue(string(*profile.Attributes.ProfileState))
	} else {
		state.ProfileState = types.StringNull()
	}
	if profile.Attributes.ProfileType != nil {
		state.ProfileType = types.StringValue(string(*profile.Attributes.ProfileType))
	} else {
		state.ProfileType = types.StringNull()
	}
	if profile.Attributes.CreatedDate != nil {
		state.CreatedDate = types.StringValue(profile.Attributes.CreatedDate.Format(time.RFC3339))
	} else {
		state.CreatedDate = types.StringNull()
	}
	if profile.Attributes.ExpirationDate != nil {
		state.ExpirationDate = types.StringValue(profile.Attributes.ExpirationDate.Format(time.RFC3339))
	} else {
		state.ExpirationDate = types.StringNull()
	}

	tflog.Debug(ctx, "Profile read successfully")

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource.
func (r *profileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating Profile resource")

	// Retrieve values from plan and current state
	var plan profileModel
	var state profileModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)

	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "profile_id", state.ID.ValueString())
	ctx = tflog.SetField(ctx, "new_name", plan.Name.ValueString())

	tflog.Debug(ctx, "Updating Profile with Apple API")

	// Update Profile name (only field that can be updated)
	profile, err := r.client.UpdateProfile(
		state.ID.ValueString(),
		plan.Name.ValueString(),
		nil,
	)
	if err != nil {
		tflog.Error(ctx, "Failed to update Profile", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Updating Profile",
			fmt.Sprintf("Could not update Profile %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Update the plan with the updated values
	plan.ID = types.StringValue(profile.ID)
	plan.Platform = platformValue(profile.Attributes.Platform)
	if profile.Attributes.ProfileContent != nil {
		plan.ProfileContent = types.StringValue(*profile.Attributes.ProfileContent)
	} else {
		plan.ProfileContent = types.StringNull()
	}
	if profile.Attributes.UUID != nil {
		plan.UUID = types.StringValue(*profile.Attributes.UUID)
	} else {
		plan.UUID = types.StringNull()
	}
	if profile.Attributes.ProfileState != nil {
		plan.ProfileState = types.StringValue(string(*profile.Attributes.ProfileState))
	} else {
		plan.ProfileState = types.StringNull()
	}
	// profile_type is configured, so leave the planned value in place when Apple
	// omits it: overwriting with null would contradict the config after apply.
	if profile.Attributes.ProfileType != nil {
		plan.ProfileType = types.StringValue(string(*profile.Attributes.ProfileType))
	}
	if profile.Attributes.CreatedDate != nil {
		plan.CreatedDate = types.StringValue(profile.Attributes.CreatedDate.Format(time.RFC3339))
	} else {
		plan.CreatedDate = types.StringNull()
	}
	if profile.Attributes.ExpirationDate != nil {
		plan.ExpirationDate = types.StringValue(profile.Attributes.ExpirationDate.Format(time.RFC3339))
	} else {
		plan.ExpirationDate = types.StringNull()
	}

	tflog.Info(ctx, "Profile updated successfully")

	// Set updated state
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource.
func (r *profileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "Deleting Profile resource")

	// Retrieve values from state
	var state profileModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "profile_id", state.ID.ValueString())

	tflog.Debug(ctx, "Deleting Profile with Apple API")

	// Delete existing Profile
	err := r.client.DeleteProfile(state.ID.ValueString(), nil)
	if err != nil {
		// If the resource is already gone, that's okay
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			tflog.Warn(ctx, "Profile already deleted")
			return
		}

		tflog.Error(ctx, "Failed to delete Profile", map[string]interface{}{
			"error": err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Deleting Profile",
			fmt.Sprintf("Could not delete Profile %s: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Profile deleted successfully")
}

// Configure adds the provider configured client to the resource.
func (r *profileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an existing resource by ID or name.
func (r *profileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing Profile resource", map[string]interface{}{
		"import_id": req.ID,
	})

	// Try to import by Profile ID first
	profile, err := r.client.GetProfile(req.ID)
	if err != nil {
		// If not found by ID, try by name
		profile, err = r.client.GetProfileByName(req.ID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Importing Profile",
				fmt.Sprintf("Could not import Profile '%s': %s\n\nThe import ID can be either:\n- Apple Profile ID (e.g., 'ABC123XYZ')\n- Profile name (e.g., 'My iOS Profile')", req.ID, err.Error()),
			)
			return
		}
	}

	// Set the profile data in state
	var state profileModel
	state.ID = types.StringValue(profile.ID)
	state.Name = types.StringValue(profile.Attributes.Name)
	state.Platform = platformValue(profile.Attributes.Platform)

	// For import, we need to set relationships to empty lists since we don't have them from the API response
	state.Certificates, _ = types.ListValue(types.StringType, []attr.Value{})
	state.Devices, _ = types.ListValue(types.StringType, []attr.Value{})
	state.BundleID = types.StringValue("") // Will be resolved from relationships if available

	if profile.Attributes.ProfileContent != nil {
		state.ProfileContent = types.StringValue(*profile.Attributes.ProfileContent)
	} else {
		state.ProfileContent = types.StringNull()
	}
	if profile.Attributes.UUID != nil {
		state.UUID = types.StringValue(*profile.Attributes.UUID)
	} else {
		state.UUID = types.StringNull()
	}
	if profile.Attributes.ProfileState != nil {
		state.ProfileState = types.StringValue(string(*profile.Attributes.ProfileState))
	} else {
		state.ProfileState = types.StringNull()
	}
	if profile.Attributes.ProfileType != nil {
		state.ProfileType = types.StringValue(string(*profile.Attributes.ProfileType))
	} else {
		state.ProfileType = types.StringNull()
	}
	if profile.Attributes.CreatedDate != nil {
		state.CreatedDate = types.StringValue(profile.Attributes.CreatedDate.Format(time.RFC3339))
	} else {
		state.CreatedDate = types.StringNull()
	}
	if profile.Attributes.ExpirationDate != nil {
		state.ExpirationDate = types.StringValue(profile.Attributes.ExpirationDate.Format(time.RFC3339))
	} else {
		state.ExpirationDate = types.StringNull()
	}

	tflog.Info(ctx, "Profile imported successfully", map[string]interface{}{
		"profile_id":   profile.ID,
		"profile_name": profile.Attributes.Name,
	})

	// Set state
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
