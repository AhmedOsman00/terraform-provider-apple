// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package beta

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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// testerRecord is one tester record: Apple keeps one per address per app.
type testerRecord struct {
	id, app, email, firstName string
}

// testFlight is an App Store Connect holding tester records and the
// memberships linking them to groups, kept apart the way Apple keeps them:
// removing a membership leaves the record, and a record belongs to one app.
// Every group belongs to app-1 unless groupApps says otherwise.
type testFlight struct {
	mu        sync.Mutex
	records   []testerRecord
	members   map[string][]string // group ID -> tester record IDs
	groupApps map[string]string
	requests  []string
	created   int

	// raceCreate makes the tester POST find a record that appeared after the
	// lookup, as when two resources put one person in two groups at once.
	raceCreate bool
}

func newTestFlight(t *testing.T, records ...testerRecord) (*testFlight, *betaTesterResource) {
	t.Helper()
	tf := &testFlight{records: records, members: map[string][]string{}, groupApps: map[string]string{}}

	srv := httptest.NewServer(http.HandlerFunc(tf.serve))
	t.Cleanup(srv.Close)

	client := &apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}
	return tf, &betaTesterResource{client: client}
}

func (tf *testFlight) appOf(group string) string {
	if app, ok := tf.groupApps[group]; ok {
		return app
	}
	return "app-1"
}

func (tf *testFlight) record(id string) *testerRecord {
	for i := range tf.records {
		if tf.records[i].id == id {
			return &tf.records[i]
		}
	}
	return nil
}

func (tf *testFlight) find(app, email string) *testerRecord {
	for i := range tf.records {
		if tf.records[i].app == app && strings.EqualFold(tf.records[i].email, email) {
			return &tf.records[i]
		}
	}
	return nil
}

func (r testerRecord) json() string {
	data, _ := json.Marshal(models.BetaTester{
		Type: "betaTesters",
		ID:   r.id,
		Attributes: models.BetaTesterAttributes{
			Email:      &r.email,
			FirstName:  &r.firstName,
			InviteType: strPtr("EMAIL"),
		},
	})
	return string(data)
}

func strPtr(s string) *string { return &s }

func (tf *testFlight) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	tf.mu.Lock()
	defer tf.mu.Unlock()
	tf.requests = append(tf.requests, r.Method+" "+r.URL.Path)

	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, code, detail string) {
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"errors":[{"status":"%d","code":%q,"title":%q,"detail":%q}]}`, status, code, detail, detail)
	}
	list := func(records []testerRecord) {
		parts := make([]string, 0, len(records))
		for _, record := range records {
			parts = append(parts, record.json())
		}
		fmt.Fprintf(w, `{"data":[%s],"links":{}}`, strings.Join(parts, ","))
	}

	path := r.URL.Path
	group := strings.Split(strings.TrimPrefix(path, "/v1/betaGroups/"), "/")[0]
	_, groupExists := tf.members[group]

	switch {
	case r.Method == http.MethodGet && path == "/v1/betaTesters":
		app, email := r.URL.Query().Get("filter[apps]"), r.URL.Query().Get("filter[email]")
		var found []testerRecord
		for _, record := range tf.records {
			// Without filter[apps], Apple answers with every app's record.
			if (app == "" || record.app == app) && strings.EqualFold(record.email, email) {
				found = append(found, record)
			}
		}
		list(found)

	case r.Method == http.MethodPost && path == "/v1/betaTesters":
		var request struct {
			Data struct {
				Attributes struct {
					Email     string  `json:"email"`
					FirstName *string `json:"firstName"`
				} `json:"attributes"`
				Relationships struct {
					BetaGroups struct {
						Data []models.ResourceData `json:"data"`
					} `json:"betaGroups"`
				} `json:"relationships"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &request)
		target := request.Data.Relationships.BetaGroups.Data[0].ID
		app, email := tf.appOf(target), request.Data.Attributes.Email
		if tf.raceCreate {
			tf.records = append(tf.records, testerRecord{id: "raced", app: app, email: email})
		}
		if tf.find(app, email) != nil {
			fail(http.StatusConflict, "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE", "A tester with this email already exists")
			return
		}
		tf.created++
		record := testerRecord{id: fmt.Sprintf("tester-%d", tf.created), app: app, email: email}
		if name := request.Data.Attributes.FirstName; name != nil {
			record.firstName = *name
		}
		tf.records = append(tf.records, record)
		tf.members[target] = append(tf.members[target], record.id)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"data":%s}`, record.json())

	case !groupExists:
		fail(http.StatusNotFound, "NOT_FOUND", "There is no resource of type 'betaGroups' with id '"+group+"'")

	case r.Method == http.MethodGet && path == "/v1/betaGroups/"+group:
		fmt.Fprintf(w, `{"data":{"type":"betaGroups","id":%q,"attributes":{},"relationships":{"app":{"data":{"type":"apps","id":%q}}}}}`,
			group, tf.appOf(group))

	case r.Method == http.MethodGet && path == "/v1/betaGroups/"+group+"/betaTesters":
		var in []testerRecord
		for _, id := range tf.members[group] {
			in = append(in, *tf.record(id))
		}
		list(in)

	case path == "/v1/betaGroups/"+group+"/relationships/betaTesters":
		var linkage struct {
			Data []models.ResourceData `json:"data"`
		}
		_ = json.Unmarshal(body, &linkage)
		id := linkage.Data[0].ID
		switch r.Method {
		case http.MethodPost:
			if record := tf.record(id); record == nil || record.app != tf.appOf(group) {
				fail(http.StatusConflict, "ENTITY_ERROR", "Tester(s) cannot be assigned")
				return
			}
			tf.members[group] = append(tf.members[group], id)
		case http.MethodDelete:
			kept := tf.members[group][:0]
			for _, member := range tf.members[group] {
				if member != id {
					kept = append(kept, member)
				}
			}
			tf.members[group] = kept
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		fail(http.StatusNotFound, "NOT_FOUND", "no route for "+r.Method+" "+path)
	}
}

// group creates an empty group holding the given tester record IDs.
func (tf *testFlight) group(id string, members ...string) {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	tf.members[id] = append([]string{}, members...)
}

func (tf *testFlight) membersOf(group string) []string {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	return append([]string(nil), tf.members[group]...)
}

func (tf *testFlight) writes() []string {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	var writes []string
	for _, request := range tf.requests {
		if !strings.HasPrefix(request, "GET ") {
			writes = append(writes, request)
		}
	}
	return writes
}

func testerSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&betaTesterResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp.Schema
}

func testerState(t *testing.T, s schema.Schema, model betaTesterModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("building state: %v", diags)
	}
	return state
}

func testerIn(t *testing.T, state tfsdk.State) betaTesterModel {
	t.Helper()
	var model betaTesterModel
	if diags := state.Get(context.Background(), &model); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	return model
}

func plannedTester(group, email, firstName string) betaTesterModel {
	model := betaTesterModel{
		ID:         types.StringUnknown(),
		GroupID:    types.StringValue(group),
		Email:      types.StringValue(email),
		FirstName:  types.StringNull(),
		LastName:   types.StringNull(),
		InviteType: types.StringUnknown(),
	}
	if firstName != "" {
		model.FirstName = types.StringValue(firstName)
	}
	return model
}

func (r *betaTesterResource) testCreate(t *testing.T, plan betaTesterModel) resource.CreateResponse {
	t.Helper()
	s := testerSchema(t)
	planned := testerState(t, s, plan)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planned.Raw}}, &resp)
	return resp
}

// TestBetaTesterCreateNewTester checks the common case: an address the app has
// no record for is created in the group, with the configured name.
func TestBetaTesterCreateNewTester(t *testing.T) {
	tf, r := newTestFlight(t)
	tf.group("group-1")

	resp := r.testCreate(t, plannedTester("group-1", "ada@example.com", "Ada"))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if got := tf.writes(); len(got) != 1 || got[0] != "POST /v1/betaTesters" {
		t.Errorf("writes = %v, want one tester POST", got)
	}
	created := testerIn(t, resp.State)
	if created.ID.ValueString() != "tester-1" || created.InviteType.ValueString() != "EMAIL" {
		t.Errorf("state = id %v, invite type %v; want tester-1, EMAIL", created.ID, created.InviteType)
	}
	if got := tf.membersOf("group-1"); len(got) != 1 || got[0] != "tester-1" {
		t.Errorf("group-1 members = %v, want tester-1", got)
	}
}

// TestBetaTesterCreateLinksTheAppsExistingRecord checks that an address the
// app already holds, in another group, is linked rather than POSTed again, and
// that a configured name Apple will not apply is reported.
func TestBetaTesterCreateLinksTheAppsExistingRecord(t *testing.T) {
	tf, r := newTestFlight(t, testerRecord{id: "rec-ada", app: "app-1", email: "Ada@Example.com", firstName: "Ada"})
	tf.group("group-1")
	tf.group("group-2", "rec-ada")

	resp := r.testCreate(t, plannedTester("group-1", "ada@example.com", "Augusta"))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if got := tf.writes(); len(got) != 1 || got[0] != "POST /v1/betaGroups/group-1/relationships/betaTesters" {
		t.Errorf("writes = %v, want only the membership linked", got)
	}
	if got := testerIn(t, resp.State).ID.ValueString(); got != "rec-ada" {
		t.Errorf("id = %q, want the existing record rec-ada", got)
	}
	if warnings := resp.Diagnostics.Warnings(); len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), "first_name") {
		t.Errorf("warnings = %v, want one about first_name not being applied", warnings)
	}
}

// TestBetaTesterCreateIgnoresAnotherAppsRecord checks that the lookup is
// scoped to the group's app. Linking another app's record for the same address
// is what Apple answers with "Tester(s) cannot be assigned".
func TestBetaTesterCreateIgnoresAnotherAppsRecord(t *testing.T) {
	tf, r := newTestFlight(t, testerRecord{id: "rec-other", app: "app-2", email: "ada@example.com"})
	tf.group("group-1")

	resp := r.testCreate(t, plannedTester("group-1", "ada@example.com", ""))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if got := tf.writes(); len(got) != 1 || got[0] != "POST /v1/betaTesters" {
		t.Errorf("writes = %v, want a new record for app-1", got)
	}
}

// TestBetaTesterCreateRefusesAnExistingMembership checks that a tester already
// in the group is not adopted: a later destroy would then remove somebody the
// configuration never added.
func TestBetaTesterCreateRefusesAnExistingMembership(t *testing.T) {
	tf, r := newTestFlight(t, testerRecord{id: "rec-ada", app: "app-1", email: "ada@example.com"})
	tf.group("group-1", "rec-ada")

	resp := r.testCreate(t, plannedTester("group-1", "ada@example.com", ""))

	errs := resp.Diagnostics.Errors()
	if len(errs) != 1 || errs[0].Summary() != "Beta Tester Already In Group" {
		t.Fatalf("diagnostics = %v, want 'Beta Tester Already In Group'", resp.Diagnostics)
	}
	if !strings.Contains(errs[0].Detail(), "group-1/ada@example.com") {
		t.Errorf("error = %q, want it to name the import ID", errs[0].Detail())
	}
	if got := tf.writes(); len(got) != 0 {
		t.Errorf("writes = %v, want none", got)
	}
}

// TestBetaTesterCreateLinksARecordThatWonTheRace checks a DUPLICATE from the
// create: the record appeared after the lookup, so it is linked instead.
func TestBetaTesterCreateLinksARecordThatWonTheRace(t *testing.T) {
	tf, r := newTestFlight(t)
	tf.group("group-1")
	tf.raceCreate = true

	resp := r.testCreate(t, plannedTester("group-1", "ada@example.com", ""))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if got := testerIn(t, resp.State).ID.ValueString(); got != "raced" {
		t.Errorf("id = %q, want the record that won the race", got)
	}
	if got := tf.membersOf("group-1"); len(got) != 1 || got[0] != "raced" {
		t.Errorf("group-1 members = %v, want raced", got)
	}
}

// TestBetaTesterRead checks that membership, not the record, is what is read.
func TestBetaTesterRead(t *testing.T) {
	cases := []struct {
		name    string
		members []string
		group   bool
		kept    bool
	}{
		{name: "a member is kept", members: []string{"rec-ada"}, group: true, kept: true},
		{name: "a tester taken out of the group is gone", group: true},
		{name: "a deleted group takes the membership with it"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tf, r := newTestFlight(t, testerRecord{id: "rec-ada", app: "app-1", email: "Ada@Example.com"})
			if tc.group {
				tf.group("group-1", tc.members...)
			}
			s := testerSchema(t)
			model := plannedTester("group-1", "ada@example.com", "")
			model.ID = types.StringValue("rec-ada")
			model.InviteType = types.StringValue("EMAIL")
			state := testerState(t, s, model)

			resp := resource.ReadResponse{State: state}
			r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("Read: %v", resp.Diagnostics)
			}
			if !tc.kept {
				if !resp.State.Raw.IsNull() {
					t.Errorf("state = %v, want it removed", resp.State.Raw)
				}
				return
			}
			if got := testerIn(t, resp.State).Email.ValueString(); got != "ada@example.com" {
				t.Errorf("email = %q, want the configured address: Apple's capitalisation is not adopted", got)
			}
		})
	}
}

// TestBetaTesterDeleteWithdrawsOnlyTheMembership checks that destroy removes
// the tester from the group and never deletes the record, which would remove
// the person from every group of the app.
func TestBetaTesterDeleteWithdrawsOnlyTheMembership(t *testing.T) {
	for name, groupExists := range map[string]bool{"group exists": true, "group already deleted": false} {
		t.Run(name, func(t *testing.T) {
			tf, r := newTestFlight(t, testerRecord{id: "rec-ada", app: "app-1", email: "ada@example.com"})
			if groupExists {
				tf.group("group-1", "rec-ada")
			}
			tf.group("group-2", "rec-ada")
			model := plannedTester("group-1", "ada@example.com", "")
			model.ID = types.StringValue("rec-ada")
			model.InviteType = types.StringValue("EMAIL")

			var resp resource.DeleteResponse
			r.Delete(context.Background(), resource.DeleteRequest{State: testerState(t, testerSchema(t), model)}, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("Delete: %v", resp.Diagnostics)
			}
			for _, write := range tf.writes() {
				if strings.HasPrefix(write, "DELETE /v1/betaTesters/") {
					t.Errorf("sent %s, which removes the person from every group", write)
				}
			}
			if got := tf.membersOf("group-1"); len(got) != 0 {
				t.Errorf("group-1 members = %v, want none", got)
			}
			if got := tf.membersOf("group-2"); len(got) != 1 {
				t.Errorf("group-2 members = %v, want rec-ada untouched", got)
			}
		})
	}
}

// TestBetaTesterImport checks the composite import ID and that the names are
// left null, since adopting them would plan a replacement.
func TestBetaTesterImport(t *testing.T) {
	tf, r := newTestFlight(t, testerRecord{id: "rec-ada", app: "app-1", email: "ada@example.com", firstName: "Ada"})
	tf.group("group-1", "rec-ada")
	s := testerSchema(t)
	empty := func() resource.ImportStateResponse {
		return resource.ImportStateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}}
	}

	resp := empty()
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "group-1/ada@example.com"}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState: %v", resp.Diagnostics)
	}
	imported := testerIn(t, resp.State)
	if imported.ID.ValueString() != "rec-ada" || imported.GroupID.ValueString() != "group-1" {
		t.Errorf("imported id %v, group %v; want rec-ada, group-1", imported.ID, imported.GroupID)
	}
	if !imported.FirstName.IsNull() {
		t.Errorf("first_name = %v, want null", imported.FirstName)
	}

	resp = empty()
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "rec-ada"}, &resp)
	if errs := resp.Diagnostics.Errors(); len(errs) != 1 || errs[0].Summary() != "Invalid Import ID" {
		t.Errorf("bare ID diagnostics = %v, want 'Invalid Import ID'", resp.Diagnostics)
	}
}
