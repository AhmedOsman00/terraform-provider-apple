// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWorkflowResourceLifecycle(t *testing.T) {
	ctx := context.Background()
	var stored map[string]any
	var deletedFromCollection bool
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		methods = append(methods, req.Method)
		if req.Method == "DELETE" {
			deletedFromCollection = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if req.URL.Path == "/v1/ciProducts/product/workflows" {
			active := []any{}
			if stored != nil && !deletedFromCollection {
				active = append(active, stored)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"data": active, "links": map[string]any{}}); err != nil {
				t.Error(err)
			}
			return
		}
		if req.Method == "POST" || req.Method == "PATCH" {
			var body struct {
				Data map[string]any `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			if stored == nil {
				stored = body.Data
			} else {
				stored["attributes"] = body.Data["attributes"]
			}
			stored["id"] = "workflow"
			attributes, ok := stored["attributes"].(map[string]any)
			if !ok {
				t.Error("missing attributes")
				return
			}
			actions, ok := attributes["actions"].([]any)
			if !ok || len(actions) != 1 {
				t.Error("missing action")
				return
			}
			action, ok := actions[0].(map[string]any)
			if !ok {
				t.Error("missing action object")
				return
			}
			if _, present := action["name"]; present {
				t.Error("request must not send Apple's computed action name")
			}
			action["name"] = "Build - iOS"
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"data": stored}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	r := &workflowResource{client: &apple.Client{HostURL: server.URL, HTTPClient: server.Client(), Token: "test"}}
	var schemaResponse resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	native := map[string]any{
		"name": "Disposable", "description": "", "isEnabled": false, "clean": false, "containerFilePath": "App.xcodeproj",
		"product_id": "product", "repository_id": "repository", "xcode_version_id": "xcode", "mac_os_version_id": "macos",
		"manualBranchStartCondition": map[string]any{"source": map[string]any{"isAllMatch": true}},
		"actions":                    []any{map[string]any{"actionType": "BUILD", "scheme": "App", "platform": "IOS", "isRequiredToPass": true}},
	}
	value, err := fromJSON(ctx, schemaResponse.Schema.Type(), native)
	if err != nil {
		t.Fatal(err)
	}
	state := tfsdk.State{Schema: schemaResponse.Schema}
	if d := state.Set(ctx, value); d.HasError() {
		t.Fatal(d)
	}
	plan := tfsdk.Plan{Schema: schemaResponse.Schema, Raw: state.Raw}
	created := resource.CreateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, &created)
	if created.Diagnostics.HasError() {
		t.Fatal(created.Diagnostics)
	}
	var object types.Object
	if d := created.State.Get(ctx, &object); d.HasError() {
		t.Fatal(d)
	}
	if objectString(object, "id") != "workflow" {
		t.Fatal(object)
	}
	read := resource.ReadResponse{State: created.State}
	r.Read(ctx, resource.ReadRequest{State: created.State}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	var refreshed types.Object
	if d := read.State.Get(ctx, &refreshed); d.HasError() {
		t.Fatal(d)
	}
	if !refreshed.Equal(object) {
		t.Fatalf("read changed configured state: %v", refreshed)
	}
	values := refreshed.Attributes()
	values["description"] = types.StringValue("Updated")
	cleared, err := fromJSON(ctx, refreshed.AttributeTypes(ctx)["manual_branch_start_condition"], nil)
	if err != nil {
		t.Fatal(err)
	}
	values["manual_branch_start_condition"] = cleared
	updatedPlan := types.ObjectValueMust(refreshed.AttributeTypes(ctx), values)
	wire, err := workflowFromPlan(updatedPlan)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(wire.Attributes)
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(body, &encoded); err != nil {
		t.Fatal(err)
	}
	if value, present := encoded["manualBranchStartCondition"]; !present || string(value) != "null" {
		t.Fatalf("removed trigger must serialize as explicit null: %s", body)
	}
	if d := state.Set(ctx, updatedPlan); d.HasError() {
		t.Fatal(d)
	}
	updated := resource.UpdateResponse{State: read.State}
	r.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: schemaResponse.Schema, Raw: state.Raw}, State: read.State}, &updated)
	if updated.Diagnostics.HasError() {
		t.Fatal(updated.Diagnostics)
	}
	imported := resource.ImportStateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "workflow"}, &imported)
	if imported.Diagnostics.HasError() {
		t.Fatal(imported.Diagnostics)
	}
	var importedObject types.Object
	if d := imported.State.Get(ctx, &importedObject); d.HasError() {
		t.Fatal(d)
	}
	if !importedObject.Equal(updatedPlan) {
		t.Fatalf("import differs: %v", importedObject)
	}
	deleted := resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: updated.State}, &deleted)
	if deleted.Diagnostics.HasError() {
		t.Fatal(deleted.Diagnostics)
	}
	afterDelete := resource.ReadResponse{State: updated.State}
	r.Read(ctx, resource.ReadRequest{State: updated.State}, &afterDelete)
	if afterDelete.Diagnostics.HasError() {
		t.Fatal(afterDelete.Diagnostics)
	}
	if !afterDelete.State.Raw.IsNull() {
		t.Fatal("soft-deleted workflow must be removed from state")
	}
	historical, err := r.client.GetCiWorkflow("workflow")
	if err != nil || historical.ID != "workflow" {
		t.Fatal("fixture must retain deleted workflow by ID", err)
	}
	rejected := resource.ImportStateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "workflow"}, &rejected)
	if !rejected.Diagnostics.HasError() {
		t.Fatal("import must reject a soft-deleted workflow")
	}
	if fmt.Sprint(methods) != "[POST GET GET PATCH GET GET DELETE GET GET GET GET]" {
		t.Fatal(methods)
	}
}
func TestWorkflowUnknownAndNullValues(t *testing.T) {
	ctx := context.Background()
	r := &workflowResource{}
	var schemaResponse resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	// fromJSON must supply every null nested attribute, rather than an empty object.
	value, err := fromJSON(ctx, schemaResponse.Schema.Type(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	object, ok := value.(types.Object)
	if !ok {
		t.Fatal("expected object")
	}
	fields := object.Attributes()
	fields["name"] = types.StringUnknown()
	unknown := types.ObjectValueMust(object.AttributeTypes(ctx), fields)
	if _, err := workflowFromPlan(unknown); err == nil {
		t.Fatal("unknown input should not be serialized as null")
	}
	if _, err := toJSON(types.ListValueMust(types.StringType, []attr.Value{types.StringUnknown()})); err == nil {
		t.Fatal("unknown list element should fail")
	}
}
func TestWorkflowReadDefaultsAndDrift(t *testing.T) {
	ctx := context.Background()
	typ := types.ObjectType{AttrTypes: map[string]attr.Type{"test_configuration": types.ObjectType{AttrTypes: map[string]attr.Type{"test_plan_name": types.StringType}}, "name": types.StringType}}
	prior, err := fromJSON(ctx, typ, map[string]any{"testConfiguration": map[string]any{}, "name": "Before"})
	if err != nil {
		t.Fatal(err)
	}
	current, err := fromJSON(ctx, typ, map[string]any{"testConfiguration": map[string]any{"testPlanName": ""}, "name": "After"})
	if err != nil {
		t.Fatal(err)
	}
	reconciled := keepUnsetNested(prior, current, 0)
	expected, err := fromJSON(ctx, typ, map[string]any{"testConfiguration": map[string]any{}, "name": "After"})
	if err != nil {
		t.Fatal(err)
	}
	if !reconciled.Equal(expected) {
		t.Fatalf("got %v want %v", reconciled, expected)
	}
}
