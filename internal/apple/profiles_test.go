// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// TestCreateProfileRequestBody pins the shape of POST /v1/profiles.
//
// Apple requires profileType on create and does not accept platform there --
// the type encodes the platform. Sending platform instead silently produced
// profiles of no usable distribution method, so the body is asserted directly
// rather than left to the credential-gated acceptance tier.
func TestCreateProfileRequestBody(t *testing.T) {
	var got map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/profiles" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"profiles","id":"PROF1","attributes":{
			"name":"Test Profile","platform":"IOS","profileType":"IOS_APP_STORE","profileState":"ACTIVE"}}}`))
	}))
	defer srv.Close()

	client := newTestClient(srv)

	profile, err := client.CreateProfile(
		"Test Profile",
		models.ProfileTypeAppStore,
		"BUNDLE1",
		[]string{"CERT1"},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("CreateProfile returned error: %v", err)
	}

	attrs, ok := got["data"].(map[string]any)["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("request body has no data.attributes: %#v", got)
	}

	if attrs["profileType"] != "IOS_APP_STORE" {
		t.Errorf("profileType = %v, want IOS_APP_STORE", attrs["profileType"])
	}
	if attrs["name"] != "Test Profile" {
		t.Errorf("name = %v, want Test Profile", attrs["name"])
	}
	if _, present := attrs["platform"]; present {
		t.Errorf("platform must not be sent on create, got %v", attrs["platform"])
	}

	rels, ok := got["data"].(map[string]any)["relationships"].(map[string]any)
	if !ok {
		t.Fatalf("request body has no data.relationships: %#v", got)
	}
	bundle, ok := rels["bundleId"].(map[string]any)["data"].(map[string]any)
	if !ok {
		t.Fatalf("bundleId relationship is not a to-one identifier: %#v", rels["bundleId"])
	}
	if bundle["id"] != "BUNDLE1" || bundle["type"] != "bundleIds" {
		t.Errorf("bundleId relationship = %#v, want id BUNDLE1 of type bundleIds", bundle)
	}

	// To-many relationships must be {"data":[...]}, not an array of {"data":{...}}.
	certWrapper, ok := rels["certificates"].(map[string]any)
	if !ok {
		t.Fatalf("certificates relationship is not an object with a data array: %#v", rels["certificates"])
	}
	certs, ok := certWrapper["data"].([]any)
	if !ok {
		t.Fatalf("certificates.data is not an array: %#v", certWrapper["data"])
	}
	if len(certs) != 1 {
		t.Fatalf("certificates.data has %d entries, want 1", len(certs))
	}
	cert, ok := certs[0].(map[string]any)
	if !ok {
		t.Fatalf("certificates.data[0] is not an object: %#v", certs[0])
	}
	if cert["id"] != "CERT1" || cert["type"] != "certificates" {
		t.Errorf("certificates.data[0] = %#v, want id CERT1 of type certificates", cert)
	}

	// An App Store profile has no devices; the key must be absent, not empty.
	if _, present := rels["devices"]; present {
		t.Errorf("devices relationship must be omitted when no devices are given, got %#v", rels["devices"])
	}

	if profile.ID != "PROF1" {
		t.Errorf("profile.ID = %q, want PROF1", profile.ID)
	}
	if profile.Attributes.ProfileType == nil || *profile.Attributes.ProfileType != models.ProfileTypeAppStore {
		t.Errorf("profile type not decoded from response: %#v", profile.Attributes.ProfileType)
	}
}

// TestCreateProfileTypesRoundTrip walks every profile type the provider exposes,
// confirming each reaches the wire unchanged.
func TestCreateProfileTypesRoundTrip(t *testing.T) {
	types := []models.ProfileType{
		models.ProfileTypeDevelopment,
		models.ProfileTypeAdhoc,
		models.ProfileTypeAppStore,
		models.ProfileTypeInHouse,
		models.ProfileTypeMacDev,
		models.ProfileTypeMacStore,
		models.ProfileTypeMacDirect,
		models.ProfileTypeTVOSDev,
		models.ProfileTypeTVOSAdhoc,
		models.ProfileTypeTVOSStore,
		models.ProfileTypeTVOSInHouse,
	}

	for _, want := range types {
		t.Run(string(want), func(t *testing.T) {
			var sent string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body models.Request[models.ProfileCreateRequest]
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decoding request body: %v", err)
				}
				sent = string(body.Data.Attributes.ProfileType)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"type":"profiles","id":"PROF1","attributes":{"name":"p"}}}`))
			}))
			defer srv.Close()

			if _, err := newTestClient(srv).CreateProfile("p", want, "B", []string{"C"}, nil, nil); err != nil {
				t.Fatalf("CreateProfile returned error: %v", err)
			}
			if sent != string(want) {
				t.Errorf("profileType on the wire = %q, want %q", sent, want)
			}
		})
	}
}

// TestCreateProfileWithDevices covers the to-many devices relationship, which a
// development or ad hoc profile requires.
func TestCreateProfileWithDevices(t *testing.T) {
	var got map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"profiles","id":"PROF2","attributes":{"name":"dev"}}}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).CreateProfile(
		"dev", models.ProfileTypeDevelopment, "BUNDLE1", []string{"CERT1"}, []string{"DEV1", "DEV2"}, nil,
	); err != nil {
		t.Fatalf("CreateProfile returned error: %v", err)
	}

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("request body has no data object: %#v", got)
	}
	rels, ok := data["relationships"].(map[string]any)
	if !ok {
		t.Fatalf("request body has no data.relationships: %#v", data)
	}
	wrapper, ok := rels["devices"].(map[string]any)
	if !ok {
		t.Fatalf("devices relationship is not an object with a data array: %#v", rels["devices"])
	}
	devices, ok := wrapper["data"].([]any)
	if !ok {
		t.Fatalf("devices.data is not an array: %#v", wrapper["data"])
	}
	if len(devices) != 2 {
		t.Fatalf("devices.data has %d entries, want 2", len(devices))
	}
	for i, want := range []string{"DEV1", "DEV2"} {
		d, ok := devices[i].(map[string]any)
		if !ok {
			t.Fatalf("devices.data[%d] is not an object: %#v", i, devices[i])
		}
		if d["id"] != want || d["type"] != "devices" {
			t.Errorf("devices.data[%d] = %#v, want id %s of type devices", i, d, want)
		}
	}
}

// TestCreateProfileRetriesServerError covers the intermittent 500 Apple returns
// from POST /v1/profiles for a request that is well formed. A single retry
// recovers, and the caller must not see the transient failure.
func TestCreateProfileRetriesServerError(t *testing.T) {
	var posts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/profiles":
			posts++
			if posts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred.","detail":"An unexpected error occurred on the server side."}]}`))
				return
			}

			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"type":"profiles","id":"PROF123","attributes":{"name":"Retry Profile","profileType":"IOS_APP_STORE"}}}`))

		case r.Method == http.MethodGet && r.URL.Path == "/v1/profiles":
			// The look-before-retry check: nothing was issued by the failed POST.
			_, _ = w.Write([]byte(`{"data":[]}`))

		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}

	profile, err := client.CreateProfile("Retry Profile", models.ProfileType("IOS_APP_STORE"), "BUNDLE1", []string{"CERT1"}, nil, nil)
	if err != nil {
		t.Fatalf("CreateProfile() returned error: %v", err)
	}

	if profile.ID != "PROF123" {
		t.Errorf("profile ID = %q, want PROF123", profile.ID)
	}

	if posts != 2 {
		t.Errorf("POST attempts = %d, want 2 (one failure, one retry)", posts)
	}
}

// TestCreateProfileAdoptsProfileIssuedByFailedAttempt covers the other half of
// the retry: a 500 that arrives after Apple has already created the profile.
// Retrying blindly would either duplicate it or fail on the unique name, so the
// client looks it up by name and adopts it.
func TestCreateProfileAdoptsProfileIssuedByFailedAttempt(t *testing.T) {
	var posts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/profiles":
			posts++
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred.","detail":""}]}`))

		case r.Method == http.MethodGet && r.URL.Path == "/v1/profiles":
			_, _ = w.Write([]byte(`{"data":[{"type":"profiles","id":"PROFEXISTING","attributes":{"name":"Retry Profile","profileType":"IOS_APP_STORE"}}]}`))

		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}

	profile, err := client.CreateProfile("Retry Profile", models.ProfileType("IOS_APP_STORE"), "BUNDLE1", []string{"CERT1"}, nil, nil)
	if err != nil {
		t.Fatalf("CreateProfile() returned error: %v", err)
	}

	if profile.ID != "PROFEXISTING" {
		t.Errorf("profile ID = %q, want the profile the failed attempt issued", profile.ID)
	}

	if posts != 1 {
		t.Errorf("POST attempts = %d, want 1: the lookup should stop the retry", posts)
	}
}

// TestCreateProfileDoesNotRetryClientError confirms the retry is limited to
// 5xx: a 409 is the caller's fault and repeating it only wastes time.
func TestCreateProfileDoesNotRetryClientError(t *testing.T) {
	var posts int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/profiles" {
			posts++
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"There is a problem with the request entity","detail":"A profile with this name already exists"}]}`))
			return
		}

		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := &Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}

	if _, err := client.CreateProfile("Dup", models.ProfileType("IOS_APP_STORE"), "BUNDLE1", []string{"CERT1"}, nil, nil); err == nil {
		t.Fatal("CreateProfile() succeeded, want the 409 surfaced")
	}

	if posts != 1 {
		t.Errorf("POST attempts = %d, want 1: a 409 must not be retried", posts)
	}
}
