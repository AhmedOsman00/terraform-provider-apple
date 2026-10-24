// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package ci

import (
	"context"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWorkflowActionNamesAreComputed(t *testing.T) {
	ctx := context.Background()
	var schemaResponse resource.SchemaResponse
	(&workflowResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResponse)
	value, err := fromJSON(ctx, schemaResponse.Schema.Type(), map[string]any{
		"name": "Workflow", "actions": []any{map[string]any{"actionType": "BUILD", "scheme": "App", "platform": "IOS", "isRequiredToPass": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, ok := value.(types.Object)
	if !ok {
		t.Fatal("expected plan object")
	}
	actions, ok := plan.Attributes()["actions"].(types.List)
	if !ok {
		t.Fatal("expected actions list")
	}
	action, ok := actions.Elements()[0].(types.Object)
	if !ok {
		t.Fatal("expected action object")
	}
	fields := action.Attributes()
	fields["name"] = types.StringUnknown()
	values := plan.Attributes()
	values["actions"] = types.ListValueMust(actions.ElementType(ctx), []attr.Value{types.ObjectValueMust(action.AttributeTypes(ctx), fields)})
	plan = types.ObjectValueMust(plan.AttributeTypes(ctx), values)
	wire, err := workflowFromPlan(plan)
	if err != nil {
		t.Fatalf("computed unknown action name must not block writes: %v", err)
	}
	if len(wire.Attributes.Actions) != 1 || wire.Attributes.Actions[0].Name != nil {
		t.Fatal("action name must be omitted from requests")
	}
	canonical := "Build - iOS"
	state := workflowState(plan, &models.CiWorkflow{ID: "workflow", Attributes: &models.CiWorkflowAttributes{Actions: []models.CiAction{{Name: &canonical}}}})
	resolved, ok := state.Attributes()["actions"].(types.List)
	if !ok {
		t.Fatal("expected resolved actions")
	}
	resolvedAction, ok := resolved.Elements()[0].(types.Object)
	if !ok || objectString(resolvedAction, "name") != canonical {
		t.Fatal("write must record Apple's generated name", resolved)
	}
}
