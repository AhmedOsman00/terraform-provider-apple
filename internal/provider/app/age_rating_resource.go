// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &ageRatingDeclarationResource{}
	_ resource.ResourceWithConfigure   = &ageRatingDeclarationResource{}
	_ resource.ResourceWithImportState = &ageRatingDeclarationResource{}
)

func NewAgeRatingDeclarationResource() resource.Resource {
	return &ageRatingDeclarationResource{}
}

type ageRatingDeclarationResource struct {
	client *apple.Client
}

// Metadata returns the resource type name.
func (r *ageRatingDeclarationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_age_rating_declaration"
}

// Schema defines the schema for the resource.
func (r *ageRatingDeclarationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Answers the content questionnaire behind an app's age rating.\n\n" +
			"Apple computes the rating from these answers; it is never set directly. The result appears " +
			"as `app_store_age_rating` on `apple_app_info`.\n\n" +
			"The questions fall into three groups: how often a kind of content appears (the frequency " +
			"attributes, each `NONE`, `INFREQUENT_OR_MILD`, `FREQUENT_OR_INTENSE`, `INFREQUENT` or " +
			"`FREQUENT`), yes/no facts about what the app does, and the overrides that raise the " +
			"computed rating.\n\n" +
			"~> **An unset answer is not `NONE`.** Apple leaves an omitted attribute exactly as it was, " +
			"which for a new app means unanswered — and an unanswered questionnaire blocks the " +
			"submission. Answer every question that applies rather than relying on a default. Each " +
			"attribute is `Computed` as well as `Optional`, so an answer given in App Store Connect and " +
			"not in the configuration is adopted rather than fought over.\n\n" +
			"~> **This resource adopts a record Apple already created; it does not create or destroy " +
			"one.** Apple makes the declaration with the app and publishes neither `POST` nor `DELETE` " +
			"for it. `terraform destroy` drops it from state and warns.\n\n" +
			"~> **Writes only land while a version is being prepared,** since the declaration hangs off " +
			"the app's AppInfo.\n\n" +
			"Apple revised this questionnaire: `INFREQUENT` and `FREQUENT` belong to the newer form and " +
			"`INFREQUENT_OR_MILD` and `FREQUENT_OR_INTENSE` to the original, and `age_rating_override` " +
			"was superseded by `age_rating_override_v2` (in which `SEVENTEEN_PLUS` became " +
			"`EIGHTEEN_PLUS`). Both are accepted because an existing declaration may hold either. " +
			"Attributes such as `age_assurance`, `loot_box`, `messaging_and_chat` and " +
			"`user_generated_content` exist only on the newer form.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique Apple-generated identifier for the declaration.",
				Computed:            true,
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "The Apple ID of the app this rating is for. Cannot be changed.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"app_info_id": schema.StringAttribute{
				MarkdownDescription: "The AppInfo record the declaration hangs off. Resolved by the " +
					"provider; Apple issues a new one each version cycle.",
				Computed: true,
			},
			"alcohol_tobacco_or_drug_use_or_references": schema.StringAttribute{
				MarkdownDescription: "References to or depictions of alcohol, tobacco or drug use. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"contests": schema.StringAttribute{
				MarkdownDescription: "Contests the app runs. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"gambling_simulated": schema.StringAttribute{
				MarkdownDescription: "Simulated gambling — casino games played with no real money at stake. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"guns_or_other_weapons": schema.StringAttribute{
				MarkdownDescription: "Depictions of guns or other weapons. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"horror_or_fear_themes": schema.StringAttribute{
				MarkdownDescription: "Horror or fear themes. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"mature_or_suggestive_themes": schema.StringAttribute{
				MarkdownDescription: "Mature or suggestive themes. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"medical_or_treatment_information": schema.StringAttribute{
				MarkdownDescription: "Medical or treatment information. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"profanity_or_crude_humor": schema.StringAttribute{
				MarkdownDescription: "Profanity or crude humor. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"sexual_content_graphic_and_nudity": schema.StringAttribute{
				MarkdownDescription: "Graphic sexual content and nudity. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"sexual_content_or_nudity": schema.StringAttribute{
				MarkdownDescription: "Sexual content or nudity. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"violence_cartoon_or_fantasy": schema.StringAttribute{
				MarkdownDescription: "Cartoon or fantasy violence. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"violence_realistic": schema.StringAttribute{
				MarkdownDescription: "Realistic violence. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"violence_realistic_prolonged_graphic_or_sadistic": schema.StringAttribute{
				MarkdownDescription: "Prolonged graphic or sadistic realistic violence. One of `NONE`, `INFREQUENT_OR_MILD`, " +
					"`FREQUENT_OR_INTENSE`, `INFREQUENT` or `FREQUENT`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingFrequencyValidator},
			},
			"advertising": schema.BoolAttribute{
				MarkdownDescription: "Whether the app displays advertising.",
				Optional:            true,
				Computed:            true,
			},
			"age_assurance": schema.BoolAttribute{
				MarkdownDescription: "Whether the app verifies a customer's age before showing age-restricted content.",
				Optional:            true,
				Computed:            true,
			},
			"gambling": schema.BoolAttribute{
				MarkdownDescription: "Whether the app offers real-money gambling. Apple rates any app answering `true` 18+ and restricts where it can be sold.",
				Optional:            true,
				Computed:            true,
			},
			"health_or_wellness_topics": schema.BoolAttribute{
				MarkdownDescription: "Whether the app covers health or wellness topics.",
				Optional:            true,
				Computed:            true,
			},
			"loot_box": schema.BoolAttribute{
				MarkdownDescription: "Whether the app sells loot boxes or any other randomised purchase.",
				Optional:            true,
				Computed:            true,
			},
			"messaging_and_chat": schema.BoolAttribute{
				MarkdownDescription: "Whether customers can message or chat with each other.",
				Optional:            true,
				Computed:            true,
			},
			"parental_controls": schema.BoolAttribute{
				MarkdownDescription: "Whether the app provides parental controls.",
				Optional:            true,
				Computed:            true,
			},
			"social_media": schema.BoolAttribute{
				MarkdownDescription: "Whether the app includes social media features.",
				Optional:            true,
				Computed:            true,
			},
			"social_media_age_restricted": schema.BoolAttribute{
				MarkdownDescription: "Whether those social media features are restricted by age.",
				Optional:            true,
				Computed:            true,
			},
			"unrestricted_web_access": schema.BoolAttribute{
				MarkdownDescription: "Whether the app can open arbitrary web pages — an in-app browser with no filtering. Apple rates any app answering `true` at least 17+.",
				Optional:            true,
				Computed:            true,
			},
			"user_generated_content": schema.BoolAttribute{
				MarkdownDescription: "Whether customers can post content other customers see.",
				Optional:            true,
				Computed:            true,
			}, "kids_age_band": schema.StringAttribute{
				MarkdownDescription: "The Kids Category band the app is made for: `FIVE_AND_UNDER`, " +
					"`SIX_TO_EIGHT` or `NINE_TO_ELEVEN`. Leave unset for an app that is not in the Kids " +
					"Category, which is the usual case — the Kids Category carries extra review rules.",
				Optional:   true,
				Validators: []validator.String{KidsAgeBandValidator},
			},
			"age_rating_override": schema.StringAttribute{
				MarkdownDescription: "Raises the rating Apple computed. One of `NONE`, `NINE_PLUS`, " +
					"`THIRTEEN_PLUS`, `SIXTEEN_PLUS`, `SEVENTEEN_PLUS` or `UNRATED`. An override can " +
					"only raise the rating, never lower it. Superseded by `age_rating_override_v2`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingOverrideValidator},
			},
			"age_rating_override_v2": schema.StringAttribute{
				MarkdownDescription: "The revised override, in which `SEVENTEEN_PLUS` became " +
					"`EIGHTEEN_PLUS`. One of `NONE`, `NINE_PLUS`, `THIRTEEN_PLUS`, `SIXTEEN_PLUS`, " +
					"`EIGHTEEN_PLUS` or `UNRATED`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{AgeRatingOverrideV2Validator},
			},
			"korea_age_rating_override": schema.StringAttribute{
				MarkdownDescription: "Raises the rating shown in the Korean App Store specifically. One " +
					"of `NONE`, `FIFTEEN_PLUS` or `NINETEEN_PLUS`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{KoreaAgeRatingOverrideValidator},
			},
			"developer_age_rating_info_url": schema.StringAttribute{
				MarkdownDescription: "A link to the developer's own explanation of the app's content, " +
					"shown alongside the rating in territories that require one.",
				Optional:   true,
				Validators: []validator.String{URLValidator},
			},
		},
	}
}

// Create adopts the declaration Apple created and answers the questionnaire.
func (r *ageRatingDeclarationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Info(ctx, "Creating age rating declaration resource")

	var plan ageRatingDeclarationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
//
// The editable AppInfo is resolved again rather than read by the stored ID, for
// the same reason apple_app_info does it: Apple issues a new AppInfo -- and a
// new declaration with it -- each version cycle.
func (r *ageRatingDeclarationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "Reading age rating declaration resource")

	var state ageRatingDeclarationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	info, err := r.client.GetEditableAppInfo(appID)
	if err != nil {
		if strings.Contains(err.Error(), "no editable app info") {
			tflog.Warn(ctx, "No editable app info; keeping state as-is")
			return
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "App not found, removing age rating declaration from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Age Rating Declaration",
			fmt.Sprintf("Could not resolve the app info of app '%s': %s", appID, err.Error()),
		)
		return
	}

	declaration, err := r.client.GetAgeRatingDeclaration(info.ID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			tflog.Info(ctx, "Age rating declaration not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Age Rating Declaration",
			fmt.Sprintf("Could not read the age rating declaration of app '%s': %s", appID, err.Error()),
		)
		return
	}

	state.AppInfoID = types.StringValue(info.ID)
	applyAgeRating(&state, declaration)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update patches the answers in place.
func (r *ageRatingDeclarationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "Updating age rating declaration resource")

	var plan ageRatingDeclarationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

// Delete drops the declaration from state.
//
// Apple publishes no DELETE, and un-answering the questionnaire would leave the
// app unsubmittable even if it were possible.
func (r *ageRatingDeclarationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ageRatingDeclarationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Age Rating Declaration Not Deleted",
		fmt.Sprintf("Apple publishes no DELETE endpoint for an age rating declaration, and un-answering "+
			"the questionnaire would leave app '%s' unsubmittable. Terraform has removed the resource "+
			"from state only; the answers remain as last set.", state.AppID.ValueString()),
	)

	tflog.Info(ctx, "Age rating declaration removed from state", map[string]interface{}{
		"app_id": state.AppID.ValueString(),
	})
}

// Configure adds the provider configured client to the resource.
func (r *ageRatingDeclarationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ImportState imports an app's age rating declaration.
//
// The import ID is the app ID: the declaration hangs off whichever AppInfo is
// currently editable, so the app is the only stable way to name it.
func (r *ageRatingDeclarationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Info(ctx, "Importing age rating declaration resource", map[string]interface{}{"import_id": req.ID})

	info, err := r.client.GetEditableAppInfo(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Age Rating Declaration",
			fmt.Sprintf("Could not resolve the app info of app '%s': %s\n\n"+
				"The import ID is the app's Apple ID.", req.ID, err.Error()),
		)
		return
	}

	declaration, err := r.client.GetAgeRatingDeclaration(info.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Age Rating Declaration",
			fmt.Sprintf("Could not read the age rating declaration of app '%s': %s", req.ID, err.Error()),
		)
		return
	}

	state := ageRatingDeclarationModel{
		AppID:     types.StringValue(req.ID),
		AppInfoID: types.StringValue(info.ID),
	}
	applyAgeRating(&state, declaration)

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// write resolves the editable AppInfo and patches its declaration.
func (r *ageRatingDeclarationResource) write(ctx context.Context, plan *ageRatingDeclarationModel, diags *diag.Diagnostics) {
	appID := plan.AppID.ValueString()
	ctx = tflog.SetField(ctx, "app_id", appID)

	info, err := r.client.GetEditableAppInfo(appID)
	if err != nil {
		diags.AddError(
			"No Editable App Info",
			fmt.Sprintf("Could not resolve an app info record to write for app '%s': %s", appID, err.Error()),
		)

		return
	}

	declaration, err := r.client.GetAgeRatingDeclaration(info.ID)
	if err != nil {
		diags.AddError(
			"Error Reading Age Rating Declaration",
			fmt.Sprintf("Could not read the age rating declaration of app '%s': %s\n\n"+
				"Apple creates the declaration alongside the app and publishes no endpoint to create "+
				"one, so its absence is unexpected.", appID, err.Error()),
		)

		return
	}

	ctx = tflog.SetField(ctx, "declaration_id", declaration.ID)
	tflog.Debug(ctx, "Writing age rating declaration")

	updated, err := r.client.UpdateAgeRatingDeclaration(declaration.ID, ageRatingAttributes(plan), nil)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "STATE_ERROR") || strings.Contains(errMsg, "not allowed"):
			diags.AddError(
				"Age Rating Declaration Not Editable",
				fmt.Sprintf("Apple refused the age rating change on app '%s': %s\n\n"+
					"An app info in review or already distributed is frozen. Wait for the current "+
					"review to finish.", appID, errMsg),
			)
		case strings.Contains(errMsg, "kidsAgeBand"):
			diags.AddError(
				"Kids Age Band Rejected",
				fmt.Sprintf("Apple refused the kids_age_band on app '%s': %s\n\n"+
					"The Kids Category has its own rules — an app answering anything but NONE to the "+
					"mature content questions, or offering unrestricted web access, cannot be in it.",
					appID, errMsg),
			)
		default:
			diags.AddError(
				"Error Writing Age Rating Declaration",
				fmt.Sprintf("Could not write the age rating declaration of app '%s': %s", appID, errMsg),
			)
		}
		tflog.Error(ctx, "Failed to write age rating declaration", map[string]interface{}{"error": errMsg})

		return
	}

	plan.AppInfoID = types.StringValue(info.ID)
	applyAgeRating(plan, updated)
}

// ageRatingAttributes renders the configured answers for the wire.
func ageRatingAttributes(plan *ageRatingDeclarationModel) models.AgeRatingDeclarationAttributes {
	return models.AgeRatingDeclarationAttributes{
		AlcoholTobaccoOrDrugUseOrReferences:         stringOrNil(plan.AlcoholTobaccoOrDrugUseOrReferences),
		Contests:                                    stringOrNil(plan.Contests),
		GamblingSimulated:                           stringOrNil(plan.GamblingSimulated),
		GunsOrOtherWeapons:                          stringOrNil(plan.GunsOrOtherWeapons),
		HorrorOrFearThemes:                          stringOrNil(plan.HorrorOrFearThemes),
		MatureOrSuggestiveThemes:                    stringOrNil(plan.MatureOrSuggestiveThemes),
		MedicalOrTreatmentInformation:               stringOrNil(plan.MedicalOrTreatmentInformation),
		ProfanityOrCrudeHumor:                       stringOrNil(plan.ProfanityOrCrudeHumor),
		SexualContentGraphicAndNudity:               stringOrNil(plan.SexualContentGraphicAndNudity),
		SexualContentOrNudity:                       stringOrNil(plan.SexualContentOrNudity),
		ViolenceCartoonOrFantasy:                    stringOrNil(plan.ViolenceCartoonOrFantasy),
		ViolenceRealistic:                           stringOrNil(plan.ViolenceRealistic),
		ViolenceRealisticProlongedGraphicOrSadistic: stringOrNil(plan.ViolenceRealisticProlongedGraphicOrSadistic),
		Advertising:                                 boolOrNil(plan.Advertising),
		AgeAssurance:                                boolOrNil(plan.AgeAssurance),
		Gambling:                                    boolOrNil(plan.Gambling),
		HealthOrWellnessTopics:                      boolOrNil(plan.HealthOrWellnessTopics),
		LootBox:                                     boolOrNil(plan.LootBox),
		MessagingAndChat:                            boolOrNil(plan.MessagingAndChat),
		ParentalControls:                            boolOrNil(plan.ParentalControls),
		SocialMedia:                                 boolOrNil(plan.SocialMedia),
		SocialMediaAgeRestricted:                    boolOrNil(plan.SocialMediaAgeRestricted),
		UnrestrictedWebAccess:                       boolOrNil(plan.UnrestrictedWebAccess),
		UserGeneratedContent:                        boolOrNil(plan.UserGeneratedContent),
		KidsAgeBand:                                 stringOrNil(plan.KidsAgeBand),
		AgeRatingOverride:                           stringOrNil(plan.AgeRatingOverride),
		AgeRatingOverrideV2:                         stringOrNil(plan.AgeRatingOverrideV2),
		KoreaAgeRatingOverride:                      stringOrNil(plan.KoreaAgeRatingOverride),
		DeveloperAgeRatingInfoURL:                   stringOrNil(plan.DeveloperAgeRatingInfoURL),
	}
}

// applyAgeRating copies Apple's view of the declaration into the model.
//
// Every answer is adopted, including its absence: the attributes are
// Optional+Computed, so an answer given in App Store Connect and not in the
// configuration belongs in state rather than being reported as drift forever.
//
// kids_age_band and developer_age_rating_info_url are the exceptions -- they are
// Optional alone, since an app not in the Kids Category has no band and
// adopting a null would be the same as leaving it unset.
func applyAgeRating(model *ageRatingDeclarationModel, declaration *models.AgeRatingDeclaration) {
	model.ID = types.StringValue(declaration.ID)

	model.AlcoholTobaccoOrDrugUseOrReferences = stringOrNull(declaration.Attributes.AlcoholTobaccoOrDrugUseOrReferences)
	model.Contests = stringOrNull(declaration.Attributes.Contests)
	model.GamblingSimulated = stringOrNull(declaration.Attributes.GamblingSimulated)
	model.GunsOrOtherWeapons = stringOrNull(declaration.Attributes.GunsOrOtherWeapons)
	model.HorrorOrFearThemes = stringOrNull(declaration.Attributes.HorrorOrFearThemes)
	model.MatureOrSuggestiveThemes = stringOrNull(declaration.Attributes.MatureOrSuggestiveThemes)
	model.MedicalOrTreatmentInformation = stringOrNull(declaration.Attributes.MedicalOrTreatmentInformation)
	model.ProfanityOrCrudeHumor = stringOrNull(declaration.Attributes.ProfanityOrCrudeHumor)
	model.SexualContentGraphicAndNudity = stringOrNull(declaration.Attributes.SexualContentGraphicAndNudity)
	model.SexualContentOrNudity = stringOrNull(declaration.Attributes.SexualContentOrNudity)
	model.ViolenceCartoonOrFantasy = stringOrNull(declaration.Attributes.ViolenceCartoonOrFantasy)
	model.ViolenceRealistic = stringOrNull(declaration.Attributes.ViolenceRealistic)
	model.ViolenceRealisticProlongedGraphicOrSadistic = stringOrNull(declaration.Attributes.ViolenceRealisticProlongedGraphicOrSadistic)
	model.Advertising = boolOrNull(declaration.Attributes.Advertising)
	model.AgeAssurance = boolOrNull(declaration.Attributes.AgeAssurance)
	model.Gambling = boolOrNull(declaration.Attributes.Gambling)
	model.HealthOrWellnessTopics = boolOrNull(declaration.Attributes.HealthOrWellnessTopics)
	model.LootBox = boolOrNull(declaration.Attributes.LootBox)
	model.MessagingAndChat = boolOrNull(declaration.Attributes.MessagingAndChat)
	model.ParentalControls = boolOrNull(declaration.Attributes.ParentalControls)
	model.SocialMedia = boolOrNull(declaration.Attributes.SocialMedia)
	model.SocialMediaAgeRestricted = boolOrNull(declaration.Attributes.SocialMediaAgeRestricted)
	model.UnrestrictedWebAccess = boolOrNull(declaration.Attributes.UnrestrictedWebAccess)
	model.UserGeneratedContent = boolOrNull(declaration.Attributes.UserGeneratedContent)
	model.AgeRatingOverride = stringOrNull(declaration.Attributes.AgeRatingOverride)
	model.AgeRatingOverrideV2 = stringOrNull(declaration.Attributes.AgeRatingOverrideV2)
	model.KoreaAgeRatingOverride = stringOrNull(declaration.Attributes.KoreaAgeRatingOverride)

	if declaration.Attributes.KidsAgeBand != nil {
		model.KidsAgeBand = types.StringValue(*declaration.Attributes.KidsAgeBand)
	}
	if declaration.Attributes.DeveloperAgeRatingInfoURL != nil {
		model.DeveloperAgeRatingInfoURL = types.StringValue(*declaration.Attributes.DeveloperAgeRatingInfoURL)
	}
}
