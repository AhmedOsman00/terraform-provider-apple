// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// maxScreenshots is how many screenshots Apple lets one set hold.
const maxScreenshots = 10

type fakeShot struct {
	id, checksum, state string
	data                []byte
}

type fakeSet struct {
	id, localization, displayType string
	shots                         []string
}

// screenshotStore is an App Store Connect holding screenshot sets and their
// screenshots, with the upload flow behind them: a reservation appends an
// empty screenshot to its set, the parts arrive on the asset host, and the
// commit records the checksum. Like Apple it refuses an eleventh screenshot,
// which is what makes the delete-before-upload order observable.
type screenshotStore struct {
	t        *testing.T
	srv      *httptest.Server
	mu       sync.Mutex
	sets     map[string]*fakeSet
	shots    map[string]*fakeShot
	requests []string
	created  int

	// versionState is the state of the version every localization belongs to.
	versionState string
	// rejected names the checksums Apple fails to process.
	rejected map[string]bool
}

func newScreenshotStore(t *testing.T) (*screenshotStore, *appScreenshotSetResource) {
	t.Helper()
	ss := &screenshotStore{
		t: t, sets: map[string]*fakeSet{}, shots: map[string]*fakeShot{},
		versionState: "PREPARE_FOR_SUBMISSION", rejected: map[string]bool{},
	}
	// Upload parts go to presigned HTTPS URLs only, so the store is a TLS server.
	ss.srv = httptest.NewTLSServer(http.HandlerFunc(ss.serve))
	t.Cleanup(ss.srv.Close)

	client := &apple.Client{HostURL: ss.srv.URL, HTTPClient: ss.srv.Client(), UploadHTTPClient: ss.srv.Client(), Token: "test"}
	return ss, &appScreenshotSetResource{client: client}
}

func (ss *screenshotStore) shotJSON(shot *fakeShot) string {
	checksum := "null"
	if shot.checksum != "" {
		checksum = fmt.Sprintf("%q", shot.checksum)
	}
	return fmt.Sprintf(`{"type":"appScreenshots","id":%q,"attributes":{"sourceFileChecksum":%s,"assetDeliveryState":{"state":%q}}}`,
		shot.id, checksum, shot.state)
}

func (ss *screenshotStore) setJSON(set *fakeSet) string {
	return fmt.Sprintf(`{"type":"appScreenshotSets","id":%q,"attributes":{"screenshotDisplayType":%q},`+
		`"relationships":{"appStoreVersionLocalization":{"data":{"type":"appStoreVersionLocalizations","id":%q}}}}`,
		set.id, set.displayType, set.localization)
}

func (ss *screenshotStore) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.requests = append(ss.requests, r.Method+" "+r.URL.Path)

	w.Header().Set("Content-Type", "application/json")
	fail := func(status int, detail string) {
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"errors":[{"status":"%d","title":%q,"detail":%q}]}`, status, detail, detail)
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	switch {
	// GET /v1/appStoreVersionLocalizations/{id}/appScreenshotSets
	case r.Method == http.MethodGet && len(parts) == 4 && parts[1] == "appStoreVersionLocalizations":
		var data []string
		for _, set := range ss.sets {
			if set.localization == parts[2] {
				data = append(data, ss.setJSON(set))
			}
		}
		fmt.Fprintf(w, `{"data":[%s],"links":{}}`, strings.Join(data, ","))

	// GET /v1/appStoreVersionLocalizations/{id}
	case r.Method == http.MethodGet && len(parts) == 3 && parts[1] == "appStoreVersionLocalizations":
		fmt.Fprintf(w, `{"data":{"type":"appStoreVersionLocalizations","id":%q,"attributes":{"locale":"en-US"},`+
			`"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`, parts[2])

	case r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersions/version-1":
		fmt.Fprintf(w, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"appVersionState":%q}}}`, ss.versionState)

	case r.Method == http.MethodPost && r.URL.Path == "/v1/appScreenshotSets":
		var request struct {
			Data struct {
				Attributes struct {
					ScreenshotDisplayType string `json:"screenshotDisplayType"`
				} `json:"attributes"`
				Relationships struct {
					AppStoreVersionLocalization struct {
						Data models.ResourceData `json:"data"`
					} `json:"appStoreVersionLocalization"`
				} `json:"relationships"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &request)
		ss.created++
		set := &fakeSet{
			id:           fmt.Sprintf("set-%d", ss.created),
			localization: request.Data.Relationships.AppStoreVersionLocalization.Data.ID,
			displayType:  request.Data.Attributes.ScreenshotDisplayType,
		}
		ss.sets[set.id] = set
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"data":%s}`, ss.setJSON(set))

	case len(parts) >= 3 && parts[1] == "appScreenshotSets":
		set, ok := ss.sets[parts[2]]
		if !ok {
			fail(http.StatusNotFound, "There is no resource of type 'appScreenshotSets' with id '"+parts[2]+"'")
			return
		}
		switch {
		case r.Method == http.MethodGet && len(parts) == 3:
			fmt.Fprintf(w, `{"data":%s}`, ss.setJSON(set))
		case r.Method == http.MethodDelete && len(parts) == 3:
			for _, id := range set.shots {
				delete(ss.shots, id)
			}
			delete(ss.sets, set.id)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && len(parts) == 4 && parts[3] == "appScreenshots":
			data := make([]string, 0, len(set.shots))
			for _, id := range set.shots {
				data = append(data, ss.shotJSON(ss.shots[id]))
			}
			fmt.Fprintf(w, `{"data":[%s],"links":{}}`, strings.Join(data, ","))
		case r.Method == http.MethodPatch && len(parts) == 5:
			var order models.ResourceIdentifiers
			_ = json.Unmarshal(body, &order)
			ids := make([]string, 0, len(order.Data))
			for _, item := range order.Data {
				ids = append(ids, item.ID)
			}
			if !sameMembers(ids, set.shots) {
				fail(http.StatusConflict, fmt.Sprintf("the order %v does not name the set's screenshots %v", ids, set.shots))
				return
			}
			set.shots = ids
			w.WriteHeader(http.StatusNoContent)
		default:
			fail(http.StatusNotFound, "no route")
		}

	case r.Method == http.MethodPost && r.URL.Path == "/v1/appScreenshots":
		var request models.Request[models.MediaAssetCreateRequest]
		_ = json.Unmarshal(body, &request)
		set := ss.sets[request.Data.Relationships["appScreenshotSet"].Data.ID]
		if len(set.shots) >= maxScreenshots {
			fail(http.StatusConflict, "A screenshot set can hold at most 10 screenshots")
			return
		}
		ss.created++
		shot := &fakeShot{id: fmt.Sprintf("shot-%d", ss.created), state: "AWAITING_UPLOAD"}
		ss.shots[shot.id] = shot
		set.shots = append(set.shots, shot.id)
		ops, _ := json.Marshal([]models.UploadOperation{{
			Method: "PUT", URL: ss.srv.URL + "/upload/" + shot.id, Offset: 0, Length: request.Data.Attributes.FileSize,
		}})
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"data":{"type":"appScreenshots","id":%q,"attributes":{"assetDeliveryState":{"state":"AWAITING_UPLOAD"},"uploadOperations":%s}}}`,
			shot.id, ops)

	case r.Method == http.MethodPut && len(parts) == 2 && parts[0] == "upload":
		ss.shots[parts[1]].data = body
		w.WriteHeader(http.StatusOK)

	case len(parts) == 3 && parts[1] == "appScreenshots":
		shot, ok := ss.shots[parts[2]]
		if !ok {
			fail(http.StatusNotFound, "There is no resource of type 'appScreenshots' with id '"+parts[2]+"'")
			return
		}
		switch r.Method {
		case http.MethodPatch:
			var commit models.Request[models.MediaAssetCommitRequest]
			_ = json.Unmarshal(body, &commit)
			if commit.Data.Attributes.SourceFileChecksum != apple.MediaChecksum(shot.data) {
				ss.t.Errorf("commit checksum %s does not match the bytes uploaded", commit.Data.Attributes.SourceFileChecksum)
			}
			shot.checksum = commit.Data.Attributes.SourceFileChecksum
			shot.state = models.AssetDeliveryComplete
			if ss.rejected[shot.checksum] {
				shot.state = models.AssetDeliveryFailed
			}
			fmt.Fprintf(w, `{"data":%s}`, ss.shotJSON(shot))
		case http.MethodGet:
			fmt.Fprintf(w, `{"data":%s}`, ss.shotJSON(shot))
		case http.MethodDelete:
			delete(ss.shots, shot.id)
			for _, set := range ss.sets {
				kept := set.shots[:0]
				for _, id := range set.shots {
					if id != shot.id {
						kept = append(kept, id)
					}
				}
				set.shots = kept
			}
			w.WriteHeader(http.StatusNoContent)
		}

	default:
		fail(http.StatusNotFound, "no route for "+r.Method+" "+r.URL.Path)
	}
}

func sameMembers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[string]int{}
	for _, id := range a {
		count[id]++
	}
	for _, id := range b {
		count[id]--
	}
	for _, n := range count {
		if n != 0 {
			return false
		}
	}
	return true
}

// addSet holds a set with one screenshot per content, in order, the way a
// version Apple started from the previous one does.
func (ss *screenshotStore) addSet(id, localization, displayType string, contents ...string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	set := &fakeSet{id: id, localization: localization, displayType: displayType}
	for i, content := range contents {
		shot := &fakeShot{
			id:       fmt.Sprintf("%s-old-%d", id, i),
			checksum: apple.MediaChecksum([]byte(content)),
			state:    models.AssetDeliveryComplete,
			data:     []byte(content),
		}
		ss.shots[shot.id] = shot
		set.shots = append(set.shots, shot.id)
	}
	ss.sets[id] = set
}

// failShot marks a screenshot as one Apple failed to process.
func (ss *screenshotStore) failShot(id string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.shots[id].state = models.AssetDeliveryFailed
}

// contents lists what the set holds, in order.
func (ss *screenshotStore) contents(setID string) []string {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	set, ok := ss.sets[setID]
	if !ok {
		return nil
	}
	held := make([]string, 0, len(set.shots))
	for _, id := range set.shots {
		held = append(held, string(ss.shots[id].data))
	}
	return held
}

func (ss *screenshotStore) shotIDs(setID string) []string {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return append([]string(nil), ss.sets[setID].shots...)
}

func (ss *screenshotStore) count(prefix string) int {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	n := 0
	for _, request := range ss.requests {
		if strings.HasPrefix(request, prefix) {
			n++
		}
	}
	return n
}

// writes lists every request that changes something, in order, leaving out
// the upload parts and commits that belong to a reservation.
func (ss *screenshotStore) writes() []string {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	var writes []string
	for _, request := range ss.requests {
		if strings.HasPrefix(request, "GET ") || strings.HasPrefix(request, "PUT ") || strings.HasPrefix(request, "PATCH /v1/appScreenshots/") {
			continue
		}
		writes = append(writes, request)
	}
	return writes
}

// screenshotFiles writes each content to its own file and returns the paths.
func screenshotFiles(t *testing.T, contents ...string) []string {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, len(contents))
	for i, content := range contents {
		paths[i] = filepath.Join(dir, fmt.Sprintf("%02d.png", i))
		if err := os.WriteFile(paths[i], []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func screenshotSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&appScreenshotSetResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp.Schema
}

func stringList(t *testing.T, values []string) types.List {
	t.Helper()
	if values == nil {
		return types.ListNull(types.StringType)
	}
	list, diags := types.ListValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return list
}

func checksumsOf(contents ...string) []string {
	checksums := make([]string, len(contents))
	for i, content := range contents {
		checksums[i] = apple.MediaChecksum([]byte(content))
	}
	return checksums
}

// screenshotPlan is the plan ModifyPlan would produce for these files.
func screenshotPlan(t *testing.T, id types.String, paths []string, contents []string) appScreenshotSetModel {
	t.Helper()
	return appScreenshotSetModel{
		ID:                            id,
		AppStoreVersionLocalizationID: types.StringValue("loc-1"),
		ScreenshotDisplayType:         types.StringValue("APP_IPHONE_67"),
		Screenshots:                   stringList(t, paths),
		ScreenshotIDs:                 types.ListUnknown(types.StringType),
		ScreenshotChecksums:           stringList(t, checksumsOf(contents...)),
	}
}

func screenshotState(t *testing.T, s schema.Schema, model appScreenshotSetModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &model); diags.HasError() {
		t.Fatalf("building state: %v", diags)
	}
	return state
}

func screenshotsIn(t *testing.T, state tfsdk.State) appScreenshotSetModel {
	t.Helper()
	var model appScreenshotSetModel
	if diags := state.Get(context.Background(), &model); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	return model
}

func listStrings(t *testing.T, list types.List) []string {
	t.Helper()
	var values []string
	if diags := list.ElementsAs(context.Background(), &values, false); diags.HasError() {
		t.Fatal(diags)
	}
	return values
}

func (r *appScreenshotSetResource) testCreate(t *testing.T, plan appScreenshotSetModel) resource.CreateResponse {
	t.Helper()
	s := screenshotSchema(t)
	planned := screenshotState(t, s, plan)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}}
	r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planned.Raw}}, &resp)
	return resp
}

func (r *appScreenshotSetResource) testUpdate(t *testing.T, prior, plan appScreenshotSetModel) resource.UpdateResponse {
	t.Helper()
	s := screenshotSchema(t)
	planned := screenshotState(t, s, plan)
	resp := resource.UpdateResponse{State: planned}
	r.Update(context.Background(), resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s, Raw: planned.Raw}, State: screenshotState(t, s, prior)}, &resp)
	return resp
}

// recordedSet is the state of a set holding these contents under these IDs.
func recordedSet(t *testing.T, setID string, ids []string, contents ...string) appScreenshotSetModel {
	t.Helper()
	model := screenshotPlan(t, types.StringValue(setID), screenshotFiles(t, contents...), contents)
	model.ScreenshotIDs = stringList(t, ids)
	return model
}

// TestScreenshotSetCreateStartsASet checks a new set: created, uploaded, and
// put in the configured order.
func TestScreenshotSetCreateStartsASet(t *testing.T) {
	ss, r := newScreenshotStore(t)
	contents := []string{"home", "budget", "reminders"}

	resp := r.testCreate(t, screenshotPlan(t, types.StringUnknown(), screenshotFiles(t, contents...), contents))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	created := screenshotsIn(t, resp.State)
	if created.ID.ValueString() != "set-1" {
		t.Errorf("id = %v, want set-1", created.ID)
	}
	if got := ss.contents("set-1"); strings.Join(got, ",") != "home,budget,reminders" {
		t.Errorf("Apple holds %v, want the configured order", got)
	}
	if got := listStrings(t, created.ScreenshotIDs); strings.Join(got, ",") != strings.Join(ss.shotIDs("set-1"), ",") {
		t.Errorf("screenshot_ids = %v, want Apple's %v in order", got, ss.shotIDs("set-1"))
	}
}

// TestScreenshotSetCreateAdoptsTheCopiedSet checks the set a new version
// starts with: Apple copies the previous version's, so the set exists, the
// screenshots the configuration still names are kept, and the rest go.
func TestScreenshotSetCreateAdoptsTheCopiedSet(t *testing.T) {
	ss, r := newScreenshotStore(t)
	ss.addSet("set-old", "loc-1", "APP_IPHONE_67", "home", "old-feature")
	contents := []string{"home", "new-feature"}

	resp := r.testCreate(t, screenshotPlan(t, types.StringUnknown(), screenshotFiles(t, contents...), contents))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}
	if got := screenshotsIn(t, resp.State).ID.ValueString(); got != "set-old" {
		t.Errorf("id = %q, want the adopted set-old", got)
	}
	if ss.count("POST /v1/appScreenshotSets") != 0 {
		t.Error("created a second set for a display type that already had one")
	}
	if got := ss.count("POST /v1/appScreenshots"); got != 1 {
		t.Errorf("uploaded %d screenshots, want 1: home is already there", got)
	}
	if got := ss.contents("set-old"); strings.Join(got, ",") != "home,new-feature" {
		t.Errorf("Apple holds %v, want home,new-feature", got)
	}
}

// TestScreenshotSetUpdateReordersWithoutUploading checks the promise that
// matching is by content: swapping two files uploads and deletes nothing,
// keeps both IDs, and sets the order once.
func TestScreenshotSetUpdateReordersWithoutUploading(t *testing.T) {
	ss, r := newScreenshotStore(t)
	ss.addSet("set-1", "loc-1", "APP_IPHONE_67", "home", "budget")
	ids := ss.shotIDs("set-1")
	prior := recordedSet(t, "set-1", ids, "home", "budget")
	swapped := []string{"budget", "home"}

	resp := r.testUpdate(t, prior, screenshotPlan(t, types.StringValue("set-1"), screenshotFiles(t, swapped...), swapped))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	if got := ss.writes(); len(got) != 1 || got[0] != "PATCH /v1/appScreenshotSets/set-1/relationships/appScreenshots" {
		t.Errorf("writes = %v, want only the reorder", got)
	}
	if got := listStrings(t, screenshotsIn(t, resp.State).ScreenshotIDs); strings.Join(got, ",") != ids[1]+","+ids[0] {
		t.Errorf("screenshot_ids = %v, want %v swapped", got, ids)
	}
}

// TestScreenshotSetUpdateReplacesAFullSet checks that replacing all ten
// screenshots deletes before it uploads: a set holds at most ten, so
// uploading first would be refused at the eleventh.
func TestScreenshotSetUpdateReplacesAFullSet(t *testing.T) {
	ss, r := newScreenshotStore(t)
	var old, fresh []string
	for i := 0; i < maxScreenshots; i++ {
		old = append(old, fmt.Sprintf("old-%d", i))
		fresh = append(fresh, fmt.Sprintf("new-%d", i))
	}
	ss.addSet("set-1", "loc-1", "APP_IPHONE_67", old...)
	prior := recordedSet(t, "set-1", ss.shotIDs("set-1"), old...)

	resp := r.testUpdate(t, prior, screenshotPlan(t, types.StringValue("set-1"), screenshotFiles(t, fresh...), fresh))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	if got := ss.contents("set-1"); strings.Join(got, ",") != strings.Join(fresh, ",") {
		t.Errorf("Apple holds %v, want %v", got, fresh)
	}
}

// TestScreenshotSetUpdateMatchesEachDuplicate checks that two configured
// copies of one image need two screenshots: one existing copy is kept and the
// second is uploaded.
func TestScreenshotSetUpdateMatchesEachDuplicate(t *testing.T) {
	ss, r := newScreenshotStore(t)
	ss.addSet("set-1", "loc-1", "APP_IPHONE_67", "home")
	prior := recordedSet(t, "set-1", ss.shotIDs("set-1"), "home")
	contents := []string{"home", "home"}

	resp := r.testUpdate(t, prior, screenshotPlan(t, types.StringValue("set-1"), screenshotFiles(t, contents...), contents))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	if got := ss.count("POST /v1/appScreenshots"); got != 1 {
		t.Errorf("uploaded %d, want 1", got)
	}
	if got := ss.contents("set-1"); strings.Join(got, ",") != "home,home" {
		t.Errorf("Apple holds %v, want home,home", got)
	}
}

// TestScreenshotSetUpdateReplacesAFailedScreenshot checks that a screenshot
// Apple failed to process never counts as the configured file, even when its
// checksum matches.
func TestScreenshotSetUpdateReplacesAFailedScreenshot(t *testing.T) {
	ss, r := newScreenshotStore(t)
	ss.addSet("set-1", "loc-1", "APP_IPHONE_67", "home")
	broken := ss.shotIDs("set-1")[0]
	ss.failShot(broken)
	prior := recordedSet(t, "set-1", []string{broken}, "home")
	contents := []string{"home"}

	resp := r.testUpdate(t, prior, screenshotPlan(t, types.StringValue("set-1"), screenshotFiles(t, contents...), contents))

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	if got := ss.shotIDs("set-1"); len(got) != 1 || got[0] == broken {
		t.Errorf("set holds %v, want the failed screenshot replaced", got)
	}
}

// TestScreenshotSetUpdateRecordsWhatTheSetHoldsAfterAFailure checks the state
// a failed upload leaves: what Apple holds, not the plan, so the next plan
// shows what is still missing.
func TestScreenshotSetUpdateRecordsWhatTheSetHoldsAfterAFailure(t *testing.T) {
	ss, r := newScreenshotStore(t)
	ss.addSet("set-1", "loc-1", "APP_IPHONE_67", "home", "old")
	prior := recordedSet(t, "set-1", ss.shotIDs("set-1"), "home", "old")
	ss.rejected[apple.MediaChecksum([]byte("wrong-size"))] = true
	contents := []string{"home", "wrong-size"}

	resp := r.testUpdate(t, prior, screenshotPlan(t, types.StringValue("set-1"), screenshotFiles(t, contents...), contents))

	if !resp.Diagnostics.HasError() {
		t.Fatal("Update succeeded, want the rejected upload's error")
	}
	recorded := screenshotsIn(t, resp.State)
	if got := listStrings(t, recorded.ScreenshotChecksums); strings.Join(got, ",") != strings.Join(checksumsOf("home"), ",") {
		t.Errorf("screenshot_checksums = %v, want only home's: old was deleted and wrong-size refused", got)
	}
	if got := listStrings(t, recorded.ScreenshotIDs); len(got) != 1 {
		t.Errorf("screenshot_ids = %v, want one", got)
	}
}

// TestScreenshotSetCreateKeepsAFailedSetInState checks that a set created at
// Apple is recorded even when its contents could not be written: dropping it
// would leave a set nothing ever deletes.
func TestScreenshotSetCreateKeepsAFailedSetInState(t *testing.T) {
	ss, r := newScreenshotStore(t)
	ss.rejected[apple.MediaChecksum([]byte("wrong-size"))] = true
	contents := []string{"wrong-size"}

	resp := r.testCreate(t, screenshotPlan(t, types.StringUnknown(), screenshotFiles(t, contents...), contents))

	if !resp.Diagnostics.HasError() {
		t.Fatal("Create succeeded, want the rejected upload's error")
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("no state recorded for a set Apple created")
	}
	if got := screenshotsIn(t, resp.State).ID.ValueString(); got != "set-1" {
		t.Errorf("id = %q, want set-1", got)
	}
}

// TestScreenshotSetRead checks that the checksums come from Apple, so a
// screenshot replaced by hand shows as drift, and that a deleted set is gone.
func TestScreenshotSetRead(t *testing.T) {
	ss, r := newScreenshotStore(t)
	ss.addSet("set-1", "loc-1", "APP_IPHONE_67", "changed-by-hand")
	s := screenshotSchema(t)
	state := screenshotState(t, s, recordedSet(t, "set-1", []string{"set-1-old-0"}, "home"))

	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	read := screenshotsIn(t, resp.State)
	if got := listStrings(t, read.ScreenshotChecksums); strings.Join(got, ",") != strings.Join(checksumsOf("changed-by-hand"), ",") {
		t.Errorf("screenshot_checksums = %v, want Apple's", got)
	}
	if got := listStrings(t, read.Screenshots); len(got) != 1 {
		t.Errorf("screenshots = %v, want the configured paths kept", got)
	}

	_, r2 := newScreenshotStore(t) // an account where the set no longer exists
	resp = resource.ReadResponse{State: state}
	r2.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Errorf("Read of a deleted set: state %v, diagnostics %v; want it removed", resp.State.Raw, resp.Diagnostics)
	}
}

// TestScreenshotSetDelete checks that a set is deleted while its version is
// being prepared, and only dropped from state once the version is released.
func TestScreenshotSetDelete(t *testing.T) {
	for name, tc := range map[string]struct {
		versionState string
		deleted      bool
	}{
		"version being prepared": {versionState: "PREPARE_FOR_SUBMISSION", deleted: true},
		"version released":       {versionState: "READY_FOR_DISTRIBUTION"},
	} {
		t.Run(name, func(t *testing.T) {
			ss, r := newScreenshotStore(t)
			ss.versionState = tc.versionState
			ss.addSet("set-1", "loc-1", "APP_IPHONE_67", "home")
			state := screenshotState(t, screenshotSchema(t), recordedSet(t, "set-1", ss.shotIDs("set-1"), "home"))

			var resp resource.DeleteResponse
			r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("Delete: %v", resp.Diagnostics)
			}
			if got := ss.count("DELETE /v1/appScreenshotSets/set-1") == 1; got != tc.deleted {
				t.Errorf("deleted the set = %v, want %v", got, tc.deleted)
			}
		})
	}
}

// TestScreenshotSetImport checks that the localization is read back from the
// set and the paths are left null for the configuration to supply.
func TestScreenshotSetImport(t *testing.T) {
	ss, r := newScreenshotStore(t)
	ss.addSet("set-1", "loc-1", "APP_IPHONE_67", "home", "budget")
	s := screenshotSchema(t)

	resp := resource.ImportStateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(context.Background()), nil)}}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: "set-1"}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("ImportState: %v", resp.Diagnostics)
	}
	imported := screenshotsIn(t, resp.State)
	if imported.AppStoreVersionLocalizationID.ValueString() != "loc-1" || imported.ScreenshotDisplayType.ValueString() != "APP_IPHONE_67" {
		t.Errorf("imported localization %v, display type %v", imported.AppStoreVersionLocalizationID, imported.ScreenshotDisplayType)
	}
	if !imported.Screenshots.IsNull() {
		t.Errorf("screenshots = %v, want null", imported.Screenshots)
	}
	if got := listStrings(t, imported.ScreenshotChecksums); strings.Join(got, ",") != strings.Join(checksumsOf("home", "budget"), ",") {
		t.Errorf("screenshot_checksums = %v, want Apple's two", got)
	}
}
