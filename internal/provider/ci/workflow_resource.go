// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &workflowResource{}
	_ resource.ResourceWithImportState = &workflowResource{}
)

type workflowResource struct{ client *apple.Client }

func NewCiWorkflowResource() resource.Resource { return &workflowResource{} }
func (r *workflowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ci_workflow"
}
func (r *workflowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apple.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *apple.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}
func workflowFromPlan(plan types.Object) (models.CiWorkflow, error) {
	// Omit normalized display names and record Apple's response instead.
	plan = withActionNames(plan, nil)
	values := plan.Attributes()
	attributes := make(map[string]attr.Value)
	for name, value := range values {
		if name != "id" && !strings.HasSuffix(name, "_id") {
			attributes[name] = value
		}
	}
	attributeTypes := make(map[string]attr.Type)
	for name, value := range attributes {
		attributeTypes[name] = value.Type(context.Background())
	}
	native, err := toJSON(types.ObjectValueMust(attributeTypes, attributes))
	if err != nil {
		return models.CiWorkflow{}, err
	}
	body, err := json.Marshal(native)
	if err != nil {
		return models.CiWorkflow{}, err
	}
	var wire models.CiWorkflowAttributes
	if err = json.Unmarshal(body, &wire); err != nil {
		return models.CiWorkflow{}, err
	}
	relationship := func(name, kind string) *models.ResourceIdentifier {
		return &models.ResourceIdentifier{Data: models.ResourceData{Type: kind, ID: objectString(plan, name)}}
	}
	return models.CiWorkflow{ID: objectString(plan, "id"), Type: "ciWorkflows", Attributes: &wire, Relationships: &models.CiWorkflowRelationships{
		Product: relationship("product_id", "ciProducts"), Repository: relationship("repository_id", "scmRepositories"), XcodeVersion: relationship("xcode_version_id", "ciXcodeVersions"), MacOsVersion: relationship("mac_os_version_id", "ciMacOsVersions"),
	}}, nil
}
func withActionNames(value types.Object, actions []models.CiAction) types.Object {
	values := value.Attributes()
	list, ok := values["actions"].(types.List)
	if !ok || list.IsNull() || list.IsUnknown() {
		return value
	}
	elements := list.Elements()
	for i, element := range elements {
		object, ok := element.(types.Object)
		if !ok || object.IsNull() || object.IsUnknown() {
			continue
		}
		fields := object.Attributes()
		fields["name"] = types.StringNull()
		if i < len(actions) && actions[i].Name != nil {
			fields["name"] = types.StringValue(*actions[i].Name)
		}
		elements[i] = types.ObjectValueMust(object.AttributeTypes(context.Background()), fields)
	}
	values["actions"] = types.ListValueMust(list.ElementType(context.Background()), elements)
	return types.ObjectValueMust(value.AttributeTypes(context.Background()), values)
}

func workflowState(plan types.Object, workflow *models.CiWorkflow) types.Object {
	var actions []models.CiAction
	if workflow.Attributes != nil {
		actions = workflow.Attributes.Actions
	}
	return withID(withActionNames(plan, actions), workflow.ID)
}

func withID(value types.Object, id string) types.Object {
	values := value.Attributes()
	values["id"] = types.StringValue(id)
	return types.ObjectValueMust(value.AttributeTypes(context.Background()), values)
}
func (r *workflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	workflow, err := workflowFromPlan(plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Workflow Configuration", err.Error())
		return
	}
	result, err := r.client.CreateCiWorkflow(workflow)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create CI Workflow", err.Error()+"\nConfirm Xcode Cloud is configured and the product, repository and compatible Xcode/macOS versions exist.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, workflowState(plan, result))...)
}
func (r *workflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	workflow, err := workflowFromPlan(plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Workflow Configuration", err.Error())
		return
	}
	result, err := r.client.UpdateCiWorkflow(workflow)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update CI Workflow", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, workflowState(plan, result))...)
}
func (r *workflowResource) activeWorkflow(productID, workflowID string) (bool, error) {
	workflows, err := r.client.GetCiWorkflows(productID)
	if err != nil {
		if missing(err) {
			return false, nil
		}
		return false, err
	}
	for _, workflow := range workflows {
		if workflow.ID == workflowID {
			return true, nil
		}
	}
	return false, nil
}

func missing(err error) bool {
	return strings.Contains(err.Error(), "404") || strings.Contains(strings.ToLower(err.Error()), "not found")
}
func (r *workflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	active, err := r.activeWorkflow(objectString(state, "product_id"), objectString(state, "id"))
	if err != nil {
		resp.Diagnostics.AddError("Unable to List CI Workflows", err.Error())
		return
	}
	if !active {
		resp.State.RemoveResource(ctx)
		return
	}
	workflow, err := r.client.GetCiWorkflow(objectString(state, "id"))
	if err != nil {
		if missing(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to Read CI Workflow", err.Error())
		return
	}
	value, err := workflowValue(ctx, state.Type(ctx), workflow)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Record CI Workflow", err.Error())
		return
	}
	value = keepUnsetNested(state, value, 0)
	resp.Diagnostics.Append(resp.State.Set(ctx, value)...)
}

// Apple fills optional action fields with empty strings and other defaults.
func objectString(value types.Object, name string) string {
	s, ok := value.Attributes()[name].(types.String)
	if !ok {
		return ""
	}
	return s.ValueString()
}

func keepUnsetNested(prior, current attr.Value, depth int) attr.Value {
	if current.IsNull() || current.IsUnknown() {
		return current
	}
	if prior.IsNull() || prior.IsUnknown() {
		if depth > 1 {
			return prior
		}
		return current
	}
	switch c := current.(type) {
	case types.Object:
		p, ok := prior.(types.Object)
		if !ok {
			return current
		}
		values := c.Attributes()
		for name, value := range values {
			if old, ok := p.Attributes()[name]; ok && name != "name" {
				values[name] = keepUnsetNested(old, value, depth+1)
			}
		}
		return types.ObjectValueMust(c.AttributeTypes(context.Background()), values)
	case types.List:
		p, ok := prior.(types.List)
		if !ok || len(p.Elements()) != len(c.Elements()) {
			return current
		}
		values := c.Elements()
		for i := range values {
			values[i] = keepUnsetNested(p.Elements()[i], values[i], depth+1)
		}
		return types.ListValueMust(c.ElementType(context.Background()), values)
	default:
		return current
	}
}
func workflowValue(ctx context.Context, typ attr.Type, workflow *models.CiWorkflow) (attr.Value, error) {
	body, err := json.Marshal(workflow.Attributes)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	if err = json.Unmarshal(body, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, fmt.Errorf("apple returned no workflow attributes")
	}
	values["id"] = workflow.ID
	if workflow.Relationships == nil {
		return nil, fmt.Errorf("apple returned no workflow relationships; include=product,repository,xcodeVersion,macOsVersion is required")
	}
	relationships := map[string]*models.ResourceIdentifier{"product_id": workflow.Relationships.Product, "repository_id": workflow.Relationships.Repository, "xcode_version_id": workflow.Relationships.XcodeVersion, "mac_os_version_id": workflow.Relationships.MacOsVersion}
	for name, relationship := range relationships {
		if relationship == nil || relationship.Data.ID == "" {
			return nil, fmt.Errorf("apple did not return %s", name)
		}
		values[name] = relationship.Data.ID
	}
	return fromJSON(ctx, typ, values)
}
func (r *workflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteCiWorkflow(objectString(state, "id"))
	if err != nil && !missing(err) {
		resp.Diagnostics.AddError("Unable to Delete CI Workflow", err.Error())
	}
}
func (r *workflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	workflow, err := r.client.GetCiWorkflow(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Import CI Workflow", err.Error())
		return
	}
	var schemaResponse resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	value, err := workflowValue(ctx, schemaResponse.Schema.Type(), workflow)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Import CI Workflow", err.Error())
		return
	}
	active, err := r.activeWorkflow(workflow.Relationships.Product.Data.ID, workflow.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Import CI Workflow", err.Error())
		return
	}
	if !active {
		resp.Diagnostics.AddError("Unable to Import CI Workflow", "This workflow is no longer in the product's active workflow collection. Apple can retain deleted workflows when read by ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, value)...)
}
