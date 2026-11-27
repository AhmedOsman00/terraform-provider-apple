// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// pendingInvitation is one invitation and the apps it grants sight of.
type pendingInvitation struct {
	invitation  models.UserInvitation
	visibleApps []string
}

// team is an App Store Connect holding pending invitations and members. An
// invitation that is accepted is destroyed and a member with a different ID
// takes its place, which is what makes an invitation's 404 ambiguous.
type team struct {
	mu          sync.Mutex
	invitations map[string]*pendingInvitation
	members     map[string]string // username -> user ID
	requests    []string
	bodies      map[string][]string
	created     int
}

func newTeam(t *testing.T) (*team, *userInvitationResource) {
	t.Helper()
	tm := &team{invitations: map[string]*pendingInvitation{}, members: map[string]string{}, bodies: map[string][]string{}}

	srv := httptest.NewServer(http.HandlerFunc(tm.serve))
	t.Cleanup(srv.Close)

	client := &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}
	return tm, &userInvitationResource{client: client}
}

func (tm *team) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	key := r.Method + " " + r.URL.Path

	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.requests = append(tm.requests, key)
	tm.bodies[key] = append(tm.bodies[key], string(body))

	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, code, detail string) {
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"errors":[{"status":"%d","code":%q,"title":%q,"detail":%q}]}`, status, code, detail, detail)
	}
	notFound := func() { fail(http.StatusNotFound, "NOT_FOUND", "The specified resource does not exist") }

	const invitations = "/v1/userInvitations"
	id, rest, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, invitations), "/"), "/")

	switch {
	case key == "GET /v1/users":
		username := r.URL.Query().Get("filter[username]")
		var data []string
		for name, userID := range tm.members {
			if strings.EqualFold(name, username) {
				data = append(data, fmt.Sprintf(`{"type":"users","id":%q,"attributes":{"username":%q}}`, userID, name))
			}
		}
		fmt.Fprintf(w, `{"data":[%s],"links":{}}`, strings.Join(data, ","))

	case key == "GET "+invitations:
		email := r.URL.Query().Get("filter[email]")
		var data []string
		for _, pending := range tm.invitations {
			if strings.EqualFold(*pending.invitation.Attributes.Email, email) {
				encoded, _ := json.Marshal(pending.invitation)
				data = append(data, string(encoded))
			}
		}
		fmt.Fprintf(w, `{"data":[%s],"links":{}}`, strings.Join(data, ","))

	case key == "POST "+invitations:
		var request struct {
			Data struct {
				Attributes    models.UserInvitationAttributes `json:"attributes"`
				Relationships *struct {
					VisibleApps struct {
						Data []models.ResourceData `json:"data"`
					} `json:"visibleApps"`
				} `json:"relationships"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &request)
		email := *request.Data.Attributes.Email
		for _, pending := range tm.invitations {
			if strings.EqualFold(*pending.invitation.Attributes.Email, email) {
				fail(http.StatusConflict, "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE", "This person has already been invited")
				return
			}
		}
		tm.created++
		pending := &pendingInvitation{invitation: models.UserInvitation{
			Type:       "userInvitations",
			ID:         fmt.Sprintf("inv-%d", tm.created),
			Attributes: request.Data.Attributes,
		}}
		pending.invitation.Attributes.ExpirationDate = strPtr("2026-11-30T12:00:00Z")
		if pending.invitation.Attributes.AllAppsVisible == nil {
			pending.invitation.Attributes.AllAppsVisible = boolPtr(true)
		}
		if pending.invitation.Attributes.ProvisioningAllowed == nil {
			pending.invitation.Attributes.ProvisioningAllowed = boolPtr(false)
		}
		if relationships := request.Data.Relationships; relationships != nil {
			for _, app := range relationships.VisibleApps.Data {
				pending.visibleApps = append(pending.visibleApps, app.ID)
			}
		}
		tm.invitations[pending.invitation.ID] = pending
		encoded, _ := json.Marshal(pending.invitation)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"data":%s}`, encoded)

	case strings.HasPrefix(r.URL.Path, invitations+"/"):
		pending, ok := tm.invitations[id]
		if !ok {
			notFound()
			return
		}
		switch {
		case r.Method == http.MethodGet && rest == "":
			encoded, _ := json.Marshal(pending.invitation)
			fmt.Fprintf(w, `{"data":%s}`, encoded)
		case r.Method == http.MethodGet && rest == "visibleApps":
			var data []string
			for _, app := range pending.visibleApps {
				data = append(data, fmt.Sprintf(`{"type":"apps","id":%q,"attributes":{}}`, app))
			}
			fmt.Fprintf(w, `{"data":[%s],"links":{}}`, strings.Join(data, ","))
		case r.Method == http.MethodDelete && rest == "":
			delete(tm.invitations, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			notFound()
		}

	default:
		notFound()
	}
}

// invite holds a pending invitation sent outside Terraform.
func (tm *team) invite(id, email string, roles []string, visibleApps ...string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	pending := &pendingInvitation{invitation: models.UserInvitation{
		Type: "userInvitations",
		ID:   id,
		Attributes: models.UserInvitationAttributes{
			Email:               strPtr(email),
			FirstName:           strPtr("Ada"),
			LastName:            strPtr("Lovelace"),
			Roles:               roles,
			AllAppsVisible:      boolPtr(len(visibleApps) == 0),
			ProvisioningAllowed: boolPtr(false),
			ExpirationDate:      strPtr("2026-11-30T12:00:00Z"),
		},
	}, visibleApps: visibleApps}
	tm.invitations[id] = pending
}

// accept turns the invitation into a member, the way Apple does: the
// invitation is destroyed, and the member gets an ID of its own.
func (tm *team) accept(invitationID, userID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	email := *tm.invitations[invitationID].invitation.Attributes.Email
	delete(tm.invitations, invitationID)
	tm.members[email] = userID
}

// cancel drops the invitation without a member, as a cancellation or a lapse does.
func (tm *team) cancel(invitationID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.invitations, invitationID)
}

func (tm *team) sent(key string) []string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.bodies[key]
}

func (tm *team) asked(prefix string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for _, request := range tm.requests {
		if strings.HasPrefix(request, prefix) {
			return true
		}
	}
	return false
}

func strPtr(s string) *string { return &s }

func stringSet(values ...string) types.Set {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(value))
	}
	return types.SetValueMust(types.StringType, elements)
}

func invitationSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&userInvitationResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp.Schema
}

func invitationState(t *testing.T, s schema.Schema, model userInvitationModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("building state: %v", diags)
	}
	return state
}

func emptyInvitationState(s schema.Schema) tfsdk.State {
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}
}

func invitationIn(t *testing.T, state tfsdk.State) userInvitationModel {
	t.Helper()
	var model userInvitationModel
	if diags := state.Get(context.Background(), &model); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	return model
}

// recorded is the state of a pending invitation as Create would leave it.
func recorded(id, email string, visibleApps types.Set) userInvitationModel {
	allApps := types.BoolValue(visibleApps.IsNull())
	return userInvitationModel{
		ID:                  types.StringValue(id),
		Email:               types.StringValue(email),
		FirstName:           types.StringValue("Ada"),
		LastName:            types.StringValue("Lovelace"),
		Roles:               stringSet("DEVELOPER"),
		AllAppsVisible:      allApps,
		ProvisioningAllowed: types.BoolValue(false),
		VisibleApps:         visibleApps,
		ExpirationDate:      types.StringValue("2026-11-30T12:00:00Z"),
		Accepted:            types.BoolValue(false),
		UserID:              types.StringNull(),
	}
}

func (r *userInvitationResource) testRead(t *testing.T, model userInvitationModel) resource.ReadResponse {
	t.Helper()
	state := invitationState(t, invitationSchema(t), model)
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	return resp
}

func (r *userInvitationResource) testDelete(t *testing.T, model userInvitationModel) resource.DeleteResponse {
	t.Helper()
	var resp resource.DeleteResponse
	r.Delete(context.Background(), resource.DeleteRequest{State: invitationState(t, invitationSchema(t), model)}, &resp)
	return resp
}

// TestUserInvitationCreate checks the invitation sent and the state recorded.
// Naming apps means not seeing every app, and Apple rejects a request that
// says both, so all_apps_visible goes out as false.
func TestUserInvitationCreate(t *testing.T) {
	tm, r := newTeam(t)
	s := invitationSchema(t)
	plan := recorded("", "ada@example.com", stringSet("app-1"))
	plan.ID = types.StringUnknown()
	plan.AllAppsVisible = types.BoolNull()
	plan.ProvisioningAllowed = types.BoolNull()
	plan.ExpirationDate = types.StringUnknown()
	plan.Accepted = types.BoolUnknown()
	plan.UserID = types.StringUnknown()
	planned := invitationState(t, s, plan)

	resp := resource.CreateResponse{State: emptyInvitationState(s)}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planned.Raw}}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	body := tm.sent("POST /v1/userInvitations")[0]
	if !strings.Contains(body, `"allAppsVisible":false`) || !strings.Contains(body, `"id":"app-1"`) {
		t.Errorf("POST body = %s, want allAppsVisible false and app-1 named", body)
	}

	created := invitationIn(t, resp.State)
	if created.ID.ValueString() != "inv-1" || created.Accepted.ValueBool() || !created.UserID.IsNull() {
		t.Errorf("state = id %v, accepted %v, user_id %v; want inv-1, false, null", created.ID, created.Accepted, created.UserID)
	}
	if created.ExpirationDate.ValueString() != "2026-11-30T12:00:00Z" || created.ProvisioningAllowed.IsUnknown() {
		t.Errorf("expiration %v, provisioning %v; want Apple's values adopted", created.ExpirationDate, created.ProvisioningAllowed)
	}
}

// TestUserInvitationCreateNamesTheImportsForADuplicate checks that inviting an
// address already invited says how to import it rather than repeating Apple.
func TestUserInvitationCreateNamesTheImportsForADuplicate(t *testing.T) {
	tm, r := newTeam(t)
	tm.invite("inv-old", "ada@example.com", []string{"DEVELOPER"})
	s := invitationSchema(t)
	plan := recorded("", "ada@example.com", types.SetNull(types.StringType))
	plan.ID, plan.ExpirationDate, plan.Accepted, plan.UserID = types.StringUnknown(), types.StringUnknown(), types.BoolUnknown(), types.StringUnknown()
	planned := invitationState(t, s, plan)

	resp := resource.CreateResponse{State: emptyInvitationState(s)}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planned.Raw}}, &resp)

	errs := resp.Diagnostics.Errors()
	if len(errs) != 1 || errs[0].Summary() != "Already Invited Or Already A Member" {
		t.Fatalf("diagnostics = %v, want 'Already Invited Or Already A Member'", resp.Diagnostics)
	}
	for _, want := range []string{"apple_user_invitation.<name> ada@example.com", "apple_user.<name> ada@example.com"} {
		if !strings.Contains(errs[0].Detail(), want) {
			t.Errorf("error = %q, want it to name %q", errs[0].Detail(), want)
		}
	}
}

// TestUserInvitationRead covers the three things a refresh can find, and that
// visible apps are asked for only when state holds a list.
func TestUserInvitationRead(t *testing.T) {
	t.Run("a pending invitation is refreshed", func(t *testing.T) {
		tm, r := newTeam(t)
		tm.invite("inv-1", "ada@example.com", []string{"DEVELOPER", "MARKETING"})

		resp := r.testRead(t, recorded("inv-1", "ada@example.com", types.SetNull(types.StringType)))

		if resp.Diagnostics.HasError() {
			t.Fatalf("Read: %v", resp.Diagnostics)
		}
		if got := len(invitationIn(t, resp.State).Roles.Elements()); got != 2 {
			t.Errorf("roles = %d, want both of Apple's", got)
		}
		if tm.asked("GET /v1/userInvitations/inv-1/visibleApps") {
			t.Error("asked for visible apps, which state holds no list of")
		}
	})

	t.Run("visible apps are refreshed when state holds them", func(t *testing.T) {
		tm, r := newTeam(t)
		tm.invite("inv-1", "ada@example.com", []string{"DEVELOPER"}, "app-1", "app-2")

		resp := r.testRead(t, recorded("inv-1", "ada@example.com", stringSet("app-1")))

		if resp.Diagnostics.HasError() {
			t.Fatalf("Read: %v", resp.Diagnostics)
		}
		if got := len(invitationIn(t, resp.State).VisibleApps.Elements()); got != 2 {
			t.Errorf("visible apps = %d, want Apple's 2", got)
		}
	})

	t.Run("an accepted invitation is kept and names the member", func(t *testing.T) {
		tm, r := newTeam(t)
		tm.invite("inv-1", "ada@example.com", []string{"DEVELOPER"})
		tm.accept("inv-1", "user-ada")

		resp := r.testRead(t, recorded("inv-1", "ada@example.com", types.SetNull(types.StringType)))

		if resp.Diagnostics.HasError() {
			t.Fatalf("Read: %v", resp.Diagnostics)
		}
		if resp.State.Raw.IsNull() {
			t.Fatal("state removed: the next apply would invite a member again")
		}
		kept := invitationIn(t, resp.State)
		if !kept.Accepted.ValueBool() || kept.UserID.ValueString() != "user-ada" || kept.ID.ValueString() != "inv-1" {
			t.Errorf("state = accepted %v, user_id %v, id %v; want true, user-ada, inv-1", kept.Accepted, kept.UserID, kept.ID)
		}
	})

	t.Run("a cancelled or lapsed invitation is gone", func(t *testing.T) {
		tm, r := newTeam(t)
		tm.invite("inv-1", "ada@example.com", []string{"DEVELOPER"})
		tm.cancel("inv-1")

		resp := r.testRead(t, recorded("inv-1", "ada@example.com", types.SetNull(types.StringType)))

		if resp.Diagnostics.HasError() {
			t.Fatalf("Read: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Errorf("state = %v, want it removed", resp.State.Raw)
		}
	})
}

// TestUserInvitationDelete checks that destroy cancels a pending invitation
// and never removes somebody who accepted one.
func TestUserInvitationDelete(t *testing.T) {
	t.Run("a pending invitation is cancelled", func(t *testing.T) {
		tm, r := newTeam(t)
		tm.invite("inv-1", "ada@example.com", []string{"DEVELOPER"})

		resp := r.testDelete(t, recorded("inv-1", "ada@example.com", types.SetNull(types.StringType)))

		if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 0 {
			t.Fatalf("Delete: %v", resp.Diagnostics)
		}
		if len(tm.sent("DELETE /v1/userInvitations/inv-1")) != 1 {
			t.Error("did not cancel the invitation")
		}
	})

	t.Run("an invitation accepted before the refresh is only dropped", func(t *testing.T) {
		tm, r := newTeam(t)
		state := recorded("inv-1", "ada@example.com", types.SetNull(types.StringType))
		state.Accepted, state.UserID = types.BoolValue(true), types.StringValue("user-ada")

		resp := r.testDelete(t, state)

		if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
			t.Errorf("diagnostics = %v, want one warning", resp.Diagnostics)
		}
		if tm.asked("DELETE") {
			t.Errorf("sent %v, want nothing deleted", tm.requests)
		}
	})

	t.Run("an invitation accepted since the refresh is only dropped", func(t *testing.T) {
		tm, r := newTeam(t)
		tm.invite("inv-1", "ada@example.com", []string{"DEVELOPER"})
		tm.accept("inv-1", "user-ada")

		resp := r.testDelete(t, recorded("inv-1", "ada@example.com", types.SetNull(types.StringType)))

		if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
			t.Errorf("diagnostics = %v, want one warning naming apple_user", resp.Diagnostics)
		}
		if tm.asked("DELETE /v1/users") {
			t.Error("removed the member, which this resource must never do")
		}
	})

	t.Run("an invitation that lapsed is already gone", func(t *testing.T) {
		tm, r := newTeam(t)
		tm.invite("inv-1", "ada@example.com", []string{"DEVELOPER"})
		tm.cancel("inv-1")

		resp := r.testDelete(t, recorded("inv-1", "ada@example.com", types.SetNull(types.StringType)))

		if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 0 {
			t.Errorf("diagnostics = %v, want none", resp.Diagnostics)
		}
	})
}

// TestUserInvitationImport checks both import IDs, and that visible apps are
// read only for an invitation restricted to some apps.
func TestUserInvitationImport(t *testing.T) {
	for name, tc := range map[string]struct {
		id   string
		apps []string
	}{
		"by Apple ID":                {id: "inv-1"},
		"by address, restricted":     {id: "Ada@Example.com", apps: []string{"app-1"}},
		"by address, every app seen": {id: "ada@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			tm, r := newTeam(t)
			tm.invite("inv-1", "ada@example.com", []string{"DEVELOPER"}, tc.apps...)
			s := invitationSchema(t)

			resp := resource.ImportStateResponse{State: emptyInvitationState(s)}
			r.ImportState(context.Background(), resource.ImportStateRequest{ID: tc.id}, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("ImportState: %v", resp.Diagnostics)
			}
			imported := invitationIn(t, resp.State)
			if imported.ID.ValueString() != "inv-1" || imported.Accepted.ValueBool() {
				t.Errorf("state = id %v, accepted %v; want inv-1, false", imported.ID, imported.Accepted)
			}
			if got := len(imported.VisibleApps.Elements()); got != len(tc.apps) || imported.VisibleApps.IsNull() != (len(tc.apps) == 0) {
				t.Errorf("visible apps = %v, want %v", imported.VisibleApps, tc.apps)
			}
		})
	}
}
