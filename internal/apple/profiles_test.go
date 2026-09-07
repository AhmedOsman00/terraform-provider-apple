package apple

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"terraform-provider-apple/internal/apple/models"
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
