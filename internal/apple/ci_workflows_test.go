// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCiWorkflowsPagination(t *testing.T) {
	var server *httptest.Server
	requests := 0
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/v1/ciProducts/product/workflows" {
			t.Error(r.URL.Path)
		}
		if r.URL.Query().Get("cursor") == "next" {
			if _, err := fmt.Fprint(w, `{"data":[{"id":"second","type":"ciWorkflows"}],"links":{}}`); err != nil {
				t.Error(err)
			}
			return
		}
		if r.URL.Query().Get("limit") != "200" {
			t.Error(r.URL.RawQuery)
		}
		if _, err := fmt.Fprintf(w, `{"data":[{"id":"first","type":"ciWorkflows"}],"links":{"next":%q}}`, server.URL+"/v1/ciProducts/product/workflows?cursor=next"); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client := &Client{HostURL: server.URL, HTTPClient: server.Client(), Token: "test"}
	workflows, err := client.GetCiWorkflows("product")
	if err != nil {
		t.Fatal(err)
	}
	if len(workflows) != 2 || workflows[0].ID != "first" || workflows[1].ID != "second" || requests != 2 {
		t.Fatalf("workflows=%v requests=%d", workflows, requests)
	}
}
