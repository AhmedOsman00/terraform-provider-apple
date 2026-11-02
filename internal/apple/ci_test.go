// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

func TestCiWorkflowLifecycle(t *testing.T) {
	attributes := `{"name":"Disposable","description":"","isEnabled":false,"clean":false,"containerFilePath":"App.xcodeproj","branchStartCondition":{"source":{"isAllMatch":false,"patterns":[{"pattern":"main","isPrefix":false}]},"autoCancel":true,"filesAndFoldersRule":{"mode":"START_IF_ANY_FILE_MATCHES","matchers":[{"directory":"Sources","fileExtension":"swift"}]}},"tagStartCondition":{"source":{"patterns":[{"pattern":"v","isPrefix":true}]}},"pullRequestStartCondition":{"source":{"isAllMatch":true},"destination":{"patterns":[{"pattern":"main","isPrefix":false}]}},"scheduledStartCondition":{"source":{"isAllMatch":true},"schedule":{"frequency":"WEEKLY","days":["MONDAY"],"hour":9,"minute":30,"timezone":"Europe/Warsaw"}},"manualBranchStartCondition":{"source":{"isAllMatch":true}},"manualTagStartCondition":{"source":{"isAllMatch":true}},"manualPullRequestStartCondition":{"source":{"isAllMatch":true},"destination":{"isAllMatch":true}},"actions":[{"name":"Test","actionType":"TEST","scheme":"App","platform":"IOS","isRequiredToPass":true,"testConfiguration":{"kind":"SPECIFIC_TEST_PLANS","testPlanName":"Unit","testDestinations":[{"kind":"SIMULATOR","deviceTypeIdentifier":"iphone","runtimeIdentifier":"default"}]}},{"name":"Archive","actionType":"ARCHIVE","scheme":"App","destination":"ANY_IOS_DEVICE","buildDistributionAudience":"APP_STORE_ELIGIBLE","isRequiredToPass":false}]}`
	relationships := `{"product":{"data":{"type":"ciProducts","id":"product"}},"repository":{"data":{"type":"scmRepositories","id":"repository"}},"xcodeVersion":{"data":{"type":"ciXcodeVersions","id":"xcode"}},"macOsVersion":{"data":{"type":"ciMacOsVersions","id":"macos"}}}`
	document := `{"data":{"type":"ciWorkflows","id":"workflow","attributes":` + attributes + `,"relationships":` + relationships + `}}`
	var expected models.Response[models.CiWorkflow]
	if err := json.Unmarshal([]byte(document), &expected); err != nil {
		t.Fatal(err)
	}
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("include") != "product,repository,xcodeVersion,macOsVersion" {
				t.Errorf("missing relationship includes: %s", r.URL.RawQuery)
			}
		case http.MethodPost, http.MethodPatch:
			var raw json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Error(err)
				return
			}
			var request models.Request[models.CiWorkflow]
			if err := json.Unmarshal(raw, &request); err != nil {
				t.Error(err)
				return
			}
			var wire struct {
				Data struct {
					Attributes    map[string]json.RawMessage `json:"attributes"`
					Relationships map[string]json.RawMessage `json:"relationships"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Error(err)
				return
			}
			if request.Data.Type != "ciWorkflows" {
				t.Error(request.Data.Type)
			}
			if r.Method == http.MethodPost {
				if request.Data.ID != "" {
					t.Error("create must omit id")
				}
				if !reflect.DeepEqual(request.Data.Relationships, expected.Data.Relationships) {
					t.Error("create relationships differ")
				}
			}
			if r.Method == http.MethodPatch {
				if request.Data.ID != "workflow" {
					t.Error(request.Data.ID)
				}
				if _, present := wire.Data.Relationships["product"]; present {
					t.Error("update must omit product, not send null")
				}
				if _, present := wire.Data.Relationships["repository"]; present {
					t.Error("update must omit repository, not send null")
				}
				if value, present := wire.Data.Attributes["branchStartCondition"]; !present || string(value) != "null" {
					t.Errorf("removed condition must be an explicit null; present=%t value=%s", present, value)
				}
			} else if !reflect.DeepEqual(request.Data.Attributes, expected.Data.Attributes) {
				t.Error("create attributes differ")
			}
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, document); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client := &Client{HostURL: server.URL, HTTPClient: server.Client(), Token: "test"}
	created, err := client.CreateCiWorkflow(expected.Data)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "workflow" {
		t.Fatal(created.ID)
	}
	read, err := client.GetCiWorkflow(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, &expected.Data) {
		t.Fatal("read lost workflow data")
	}
	read.Attributes.BranchStartCondition = nil
	if _, err = client.UpdateCiWorkflow(*read); err != nil {
		t.Fatal(err)
	}
	if err = client.DeleteCiWorkflow(read.ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(methods, []string{"POST", "GET", "PATCH", "DELETE"}) {
		t.Fatal(methods)
	}
}
func TestCiCataloguesPagination(t *testing.T) {
	for _, collection := range []string{"ciProducts", "scmRepositories", "ciXcodeVersions", "ciMacOsVersions"} {
		t.Run(collection, func(t *testing.T) {
			var server *httptest.Server
			requests := 0
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/v1/"+collection {
					t.Error(r.URL.Path)
				}
				if requests == 1 {
					if r.URL.Query().Get("limit") != "200" {
						t.Error(r.URL.RawQuery)
					}
					if _, err := fmt.Fprintf(w, `{"data":[{"id":"first","attributes":{"name":"First"}}],"links":{"next":%q}}`, server.URL+"/v1/"+collection+"?cursor=next"); err != nil {
						t.Error(err)
					}
				} else {
					if r.URL.Query().Get("cursor") != "next" {
						t.Error(r.URL.RawQuery)
					}
					if _, err := fmt.Fprint(w, `{"data":[{"id":"second","attributes":{"name":"Second"}}],"links":{}}`); err != nil {
						t.Error(err)
					}
				}
			}))
			defer server.Close()
			client := &Client{HostURL: server.URL, HTTPClient: server.Client(), Token: "test"}
			var ids []string
			switch collection {
			case "ciProducts":
				records, err := client.GetCiProducts()
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range records {
					ids = append(ids, r.ID)
				}
			case "scmRepositories":
				records, err := client.GetScmRepositories()
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range records {
					ids = append(ids, r.ID)
				}
			case "ciXcodeVersions":
				records, err := client.GetCiXcodeVersions()
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range records {
					ids = append(ids, r.ID)
				}
			case "ciMacOsVersions":
				records, err := client.GetCiMacOsVersions()
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range records {
					ids = append(ids, r.ID)
				}
			}
			if !reflect.DeepEqual(ids, []string{"first", "second"}) || requests != 2 {
				t.Fatalf("ids=%v requests=%d", ids, requests)
			}
		})
	}
}
func TestCiWorkflowErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusConflict} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				if _, err := fmt.Fprint(w, `{"errors":[{"code":"TEST","title":"Refused","detail":"Workflow request refused"}]}`); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client := &Client{HostURL: server.URL, HTTPClient: server.Client(), Token: "test"}
			if _, err := client.GetCiWorkflow("workflow"); err == nil {
				t.Error("GET should fail")
			}
			if _, err := client.CreateCiWorkflow(models.CiWorkflow{}); err == nil {
				t.Error("POST should fail")
			}
			if _, err := client.UpdateCiWorkflow(models.CiWorkflow{ID: "workflow"}); err == nil {
				t.Error("PATCH should fail")
			}
			if err := client.DeleteCiWorkflow("workflow"); err == nil {
				t.Error("DELETE should fail")
			}
		})
	}
}
