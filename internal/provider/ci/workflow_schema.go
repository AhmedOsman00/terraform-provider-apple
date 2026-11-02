// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func optionalString(description string, values ...string) schema.StringAttribute {
	a := schema.StringAttribute{Optional: true, MarkdownDescription: description}
	if len(values) > 0 {
		a.Validators = []validator.String{stringvalidator.OneOf(values...)}
	}
	return a
}
func patternsAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Source-control patterns to match.", Attributes: map[string]schema.Attribute{
		"is_all_match": schema.BoolAttribute{Optional: true, MarkdownDescription: "Match every branch or tag instead of using patterns."},
		"patterns": schema.ListNestedAttribute{Optional: true, MarkdownDescription: "Exact names or prefixes to match.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"pattern":   schema.StringAttribute{Required: true, MarkdownDescription: "Branch or tag name/prefix."},
			"is_prefix": schema.BoolAttribute{Required: true, MarkdownDescription: "Treat the pattern as a prefix instead of an exact name."},
		}}},
	}}
}
func filesRuleAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Restrict automatic starts by changed files.", Attributes: map[string]schema.Attribute{
		"mode": optionalString("`START_IF_ANY_FILE_MATCHES` or `DO_NOT_START_IF_ALL_FILES_MATCH`.", "START_IF_ANY_FILE_MATCHES", "DO_NOT_START_IF_ALL_FILES_MATCH"),
		"matchers": schema.ListNestedAttribute{Optional: true, MarkdownDescription: "Changed-file matchers.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"directory": optionalString("Directory to match."), "file_extension": optionalString("File extension to match."), "file_name": optionalString("File name to match."),
		}}},
	}}
}
func conditionAttribute(kind string) schema.SingleNestedAttribute {
	attributes := map[string]schema.Attribute{"source": patternsAttribute()}
	if kind == "pull_request" || kind == "manual_pull_request" {
		attributes["destination"] = patternsAttribute()
	}
	if kind == "branch" || kind == "tag" || kind == "pull_request" {
		attributes["auto_cancel"] = schema.BoolAttribute{Optional: true, MarkdownDescription: "Cancel an earlier running build when a new change matches."}
		attributes["files_and_folders_rule"] = filesRuleAttribute()
	}
	return schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "When to start builds. Removing this condition disables the trigger.", Attributes: attributes}
}
func actionAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{Required: true, MarkdownDescription: "Ordered workflow actions. Supports build, test, analyze and archive.", Validators: []validator.List{listvalidator.SizeAtLeast(1)}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"name":                        schema.StringAttribute{Computed: true, MarkdownDescription: "The action's display name, normalized by Apple — for example, `Build - iOS`."},
		"action_type":                 schema.StringAttribute{Required: true, MarkdownDescription: "`BUILD`, `TEST`, `ANALYZE` or `ARCHIVE`.", Validators: []validator.String{stringvalidator.OneOf("BUILD", "TEST", "ANALYZE", "ARCHIVE")}},
		"scheme":                      schema.StringAttribute{Required: true, MarkdownDescription: "Shared Xcode scheme."},
		"platform":                    optionalString("`MACOS`, `IOS`, `TVOS`, `WATCHOS` or `VISIONOS`.", "MACOS", "IOS", "TVOS", "WATCHOS", "VISIONOS"),
		"destination":                 optionalString("Build destination: `ANY_IOS_DEVICE`, `ANY_IOS_SIMULATOR`, `ANY_TVOS_DEVICE`, `ANY_TVOS_SIMULATOR`, `ANY_WATCHOS_DEVICE`, `ANY_WATCHOS_SIMULATOR`, `ANY_MAC`, `ANY_MAC_CATALYST`, `ANY_VISIONOS_DEVICE` or `ANY_VISIONOS_SIMULATOR`.", "ANY_IOS_DEVICE", "ANY_IOS_SIMULATOR", "ANY_TVOS_DEVICE", "ANY_TVOS_SIMULATOR", "ANY_WATCHOS_DEVICE", "ANY_WATCHOS_SIMULATOR", "ANY_MAC", "ANY_MAC_CATALYST", "ANY_VISIONOS_DEVICE", "ANY_VISIONOS_SIMULATOR"),
		"is_required_to_pass":         schema.BoolAttribute{Required: true, MarkdownDescription: "Whether this action must pass for the workflow to succeed."},
		"build_distribution_audience": optionalString("Archive eligibility: `INTERNAL_ONLY` or `APP_STORE_ELIGIBLE`. Does not distribute the build.", "INTERNAL_ONLY", "APP_STORE_ELIGIBLE"),
		"test_configuration": schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Test plans and destinations for a TEST action.", Attributes: map[string]schema.Attribute{
			"kind":           optionalString("`USE_SCHEME_SETTINGS` or `SPECIFIC_TEST_PLANS`.", "USE_SCHEME_SETTINGS", "SPECIFIC_TEST_PLANS"),
			"test_plan_name": optionalString("Test plan name when selecting a specific test plan."),
			"test_destinations": schema.ListNestedAttribute{Optional: true, MarkdownDescription: "Test devices and runtimes; discover available values with `apple_ci_xcode_versions`.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"device_type_name": optionalString("Device display name."), "device_type_identifier": optionalString("Device type identifier."), "runtime_name": optionalString("Runtime display name."), "runtime_identifier": optionalString("Runtime identifier."), "kind": optionalString("`SIMULATOR` or `MAC`.", "SIMULATOR", "MAC"),
			}}},
		}},
	}}}
}
func (r *workflowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Apple workflow ID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":                schema.StringAttribute{Required: true, MarkdownDescription: "Workflow name."},
		"description":         schema.StringAttribute{Required: true, MarkdownDescription: "Workflow description; may be empty."},
		"is_enabled":          schema.BoolAttribute{Required: true, MarkdownDescription: "Whether the workflow can start builds."},
		"clean":               schema.BoolAttribute{Required: true, MarkdownDescription: "Start builds with a clean environment."},
		"container_file_path": schema.StringAttribute{Required: true, MarkdownDescription: "Repository-relative `.xcodeproj` or `.xcworkspace` path."},
		"actions":             actionAttribute(),
	}
	for _, kind := range []string{"branch", "tag", "pull_request", "manual_branch", "manual_tag", "manual_pull_request"} {
		attributes[kind+"_start_condition"] = conditionAttribute(kind)
	}
	attributes["scheduled_start_condition"] = schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Scheduled branch builds. Omit to disable scheduled starts.", Attributes: map[string]schema.Attribute{
		"source": patternsAttribute(),
		"schedule": schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Build schedule in the named time zone.", Attributes: map[string]schema.Attribute{
			"frequency": optionalString("`WEEKLY`, `DAILY` or `HOURLY`.", "WEEKLY", "DAILY", "HOURLY"),
			"days":      schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Days: `SUNDAY`, `MONDAY`, `TUESDAY`, `WEDNESDAY`, `THURSDAY`, `FRIDAY`, `SATURDAY`.", Validators: []validator.List{listvalidator.ValueStringsAre(stringvalidator.OneOf("SUNDAY", "MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY"))}},
			"hour":      schema.Int64Attribute{Optional: true, MarkdownDescription: "Hour (0–23).", Validators: []validator.Int64{int64validator.Between(0, 23)}},
			"minute":    schema.Int64Attribute{Optional: true, MarkdownDescription: "Minute (0–59).", Validators: []validator.Int64{int64validator.Between(0, 59)}},
			"timezone":  optionalString("Time zone identifier, for example `Europe/Warsaw`."),
		}},
	}}
	for _, name := range []string{"product_id", "repository_id", "xcode_version_id", "mac_os_version_id"} {
		a := schema.StringAttribute{Required: true, MarkdownDescription: "Apple relationship ID."}
		if name == "product_id" || name == "repository_id" {
			a.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
		}
		attributes[name] = a
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Manages an Xcode Cloud workflow: its actions, start conditions and Xcode/macOS versions. The product and SCM connection must already exist in Xcode Cloud.\n\n~> **Post-actions and environment variables are not managed.** Apple's API does not expose them. Configure TestFlight distribution, notifications and environment variables in Xcode or App Store Connect. `build_distribution_audience` controls archive eligibility, not distribution.\n\nA manual test confirmed that an environment variable and an email notification survived a description-only update. Other UI-only settings and update types have not been tested.", Attributes: attributes}
}
