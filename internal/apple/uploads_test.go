// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// countingTransport counts the requests that went through it, so a test can
// tell which of the client's two networks carried a request.
type countingTransport struct {
	base  http.RoundTripper
	count atomic.Int32
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.count.Add(1)
	return t.base.RoundTrip(req)
}

// fakeAssetServer imitates App Store Connect's reservation flow and the asset
// storage host the parts are sent to. Both live on one TLS server, which is
// enough: what matters is which client sent what, not which host answered.
type fakeAssetServer struct {
	t   *testing.T
	srv *httptest.Server

	mu         sync.Mutex
	parts      map[int64][]byte
	partAuth   []string
	committed  string
	deleted    []string
	finalState string
	splitAt    int64
}

func newFakeAssetServer(t *testing.T, finalState string, splitAt int64) *fakeAssetServer {
	f := &fakeAssetServer{t: t, parts: map[int64][]byte{}, finalState: finalState, splitAt: splitAt}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)

	return f
}

func (f *fakeAssetServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == "POST" && r.URL.Path == "/v1/appScreenshots":
		var sent models.Request[models.MediaAssetCreateRequest]
		if err := json.Unmarshal(body, &sent); err != nil {
			f.t.Errorf("reservation body: %v", err)
		}
		if got := sent.Data.Relationships["appScreenshotSet"].Data.ID; got != "set1" {
			f.t.Errorf("reservation parent = %q, want set1", got)
		}
		size := sent.Data.Attributes.FileSize
		ops := []models.UploadOperation{{Method: "PUT", URL: f.srv.URL + "/upload/0", Offset: 0, Length: size,
			RequestHeaders: []models.HTTPHeader{{Name: "Content-Type", Value: "image/png"}}}}
		if f.splitAt > 0 && f.splitAt < size {
			ops = []models.UploadOperation{
				{Method: "PUT", URL: f.srv.URL + "/upload/0", Offset: 0, Length: f.splitAt},
				{Method: "PUT", URL: fmt.Sprintf("%s/upload/%d", f.srv.URL, f.splitAt), Offset: f.splitAt, Length: size - f.splitAt},
			}
		}
		opsJSON, _ := json.Marshal(ops)
		fmt.Fprintf(w, `{"data":{"type":"appScreenshots","id":"shot1","attributes":{"fileName":%q,"fileSize":%d,
			"assetDeliveryState":{"state":"AWAITING_UPLOAD"},"uploadOperations":%s}}}`,
			sent.Data.Attributes.FileName, size, opsJSON)

	case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/upload/"):
		var offset int64
		if _, err := fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/upload/"), "%d", &offset); err != nil {
			f.t.Errorf("upload path %s: %v", r.URL.Path, err)
		}
		f.parts[offset] = body
		f.partAuth = append(f.partAuth, r.Header.Get("Authorization"))
		if r.Header.Get("Content-Type") == "application/json" {
			f.t.Error("an upload part was sent with the API's JSON content type")
		}
		w.WriteHeader(http.StatusOK)

	case r.Method == "PATCH" && r.URL.Path == "/v1/appScreenshots/shot1":
		var sent models.Request[models.MediaAssetCommitRequest]
		_ = json.Unmarshal(body, &sent)
		if !sent.Data.Attributes.Uploaded {
			f.t.Error("commit did not set uploaded: true")
		}
		f.committed = sent.Data.Attributes.SourceFileChecksum
		fmt.Fprint(w, `{"data":{"type":"appScreenshots","id":"shot1","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)

	case r.Method == "GET" && r.URL.Path == "/v1/appScreenshots/shot1":
		errs := ""
		if f.finalState == models.AssetDeliveryFailed {
			errs = `,"errors":[{"code":"IMAGE_INCORRECT_DIMENSIONS","description":"wrong size"}]`
		}
		fmt.Fprintf(w, `{"data":{"type":"appScreenshots","id":"shot1","attributes":{"sourceFileChecksum":%q,
			"assetDeliveryState":{"state":%q%s}}}}`, f.committed, f.finalState, errs)

	case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/appScreenshots/"):
		f.deleted = append(f.deleted, strings.TrimPrefix(r.URL.Path, "/v1/appScreenshots/"))
		w.WriteHeader(http.StatusNoContent)

	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

// clients returns a Client whose API and upload networks are counted
// separately.
func (f *fakeAssetServer) clients() (*Client, *countingTransport, *countingTransport) {
	api := &countingTransport{base: f.srv.Client().Transport}
	upload := &countingTransport{base: f.srv.Client().Transport}

	return &Client{
		HostURL:          f.srv.URL,
		HTTPClient:       &http.Client{Transport: api},
		UploadHTTPClient: &http.Client{Transport: upload},
		Token:            "test-token",
	}, api, upload
}

func fastPolling(t *testing.T) {
	interval, attempts := assetPollInterval, assetPollAttempts
	assetPollInterval, assetPollAttempts = time.Millisecond, 3
	t.Cleanup(func() { assetPollInterval, assetPollAttempts = interval, attempts })
}

// TestUploadMediaUsesSeparateNetwork covers the reason UploadHTTPClient exists:
// the file parts go to presigned URLs through their own client, without the API
// token, while the reservation and the commit stay on the API client.
func TestUploadMediaUsesSeparateNetwork(t *testing.T) {
	fastPolling(t)
	f := newFakeAssetServer(t, models.AssetDeliveryComplete, 0)
	client, api, upload := f.clients()

	data := []byte("not really a png, but bytes are bytes")
	asset, err := client.UploadAppScreenshot("set1", "home.png", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if asset.ID != "shot1" || asset.DeliveryState() != models.AssetDeliveryComplete {
		t.Errorf("asset = %s in %q, want shot1 in COMPLETE", asset.ID, asset.DeliveryState())
	}
	if got := upload.count.Load(); got != 1 {
		t.Errorf("upload client carried %d requests, want exactly the one part", got)
	}
	// Reserve, commit, and one poll.
	if got := api.count.Load(); got != 3 {
		t.Errorf("API client carried %d requests, want 3", got)
	}
	for _, auth := range f.partAuth {
		if auth != "" {
			t.Errorf("an upload part carried an Authorization header %q", auth)
		}
	}
	if f.committed != MediaChecksum(data) {
		t.Errorf("committed checksum = %q, want %q", f.committed, MediaChecksum(data))
	}
}

// TestUploadMediaSendsEveryPart covers a file Apple splits: each operation must
// receive exactly its byte range.
func TestUploadMediaSendsEveryPart(t *testing.T) {
	fastPolling(t)
	f := newFakeAssetServer(t, models.AssetDeliveryComplete, 4)
	client, _, upload := f.clients()

	data := []byte("0123456789")
	if _, err := client.UploadAppScreenshot("set1", "a.png", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := upload.count.Load(); got != 2 {
		t.Errorf("upload client carried %d requests, want 2", got)
	}
	if string(f.parts[0]) != "0123" || string(f.parts[4]) != "456789" {
		t.Errorf("parts = %q / %q, want 0123 / 456789", f.parts[0], f.parts[4])
	}
}

// TestUploadMediaRemovesRejectedAsset covers the FAILED path: Apple's reason is
// reported and the broken asset does not stay behind in the set.
func TestUploadMediaRemovesRejectedAsset(t *testing.T) {
	fastPolling(t)
	f := newFakeAssetServer(t, models.AssetDeliveryFailed, 0)
	client, _, _ := f.clients()

	_, err := client.UploadAppScreenshot("set1", "wrong.png", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "IMAGE_INCORRECT_DIMENSIONS") {
		t.Fatalf("error = %v, want Apple's reason", err)
	}
	if len(f.deleted) != 1 || f.deleted[0] != "shot1" {
		t.Errorf("deleted = %v, want the rejected asset removed", f.deleted)
	}
}

// TestUploadPartsRefusesBadOperations covers the checks made before any byte is
// sent: a range outside the file and a non-HTTPS URL.
func TestUploadPartsRefusesBadOperations(t *testing.T) {
	client := &Client{UploadHTTPClient: &http.Client{Transport: &countingTransport{base: http.DefaultTransport}}}
	data := []byte("12345")

	cases := map[string]models.UploadOperation{
		"range past the end": {URL: "https://example.com/u", Offset: 3, Length: 5},
		"empty range":        {URL: "https://example.com/u", Offset: 0, Length: 0},
		"plain http":         {URL: "http://example.com/u", Offset: 0, Length: 5},
		"relative url":       {URL: "/u", Offset: 0, Length: 5},
	}

	for name, op := range cases {
		t.Run(name, func(t *testing.T) {
			if err := client.uploadParts([]models.UploadOperation{op}, bytes.NewReader(data), int64(len(data))); err == nil {
				t.Error("expected an error")
			}
		})
	}

	if err := client.uploadParts(nil, bytes.NewReader(data), int64(len(data))); err == nil {
		t.Error("expected an error for a reservation with no operations")
	}
}

// TestGetReviewScreenshotNullIsNotFound covers a parent with no screenshot:
// Apple answers 200 with a null data member, which must read as not found.
func TestGetReviewScreenshotNullIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/subscriptions/sub1/appStoreReviewScreenshot" &&
			r.URL.Path != "/v2/inAppPurchases/iap1/appStoreReviewScreenshot" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	if _, err := client.GetSubscriptionReviewScreenshot("sub1"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("subscription: error = %v, want not found", err)
	}
	if _, err := client.GetInAppPurchaseReviewScreenshot("iap1"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("in-app purchase: error = %v, want not found", err)
	}
}

// TestReorderAppScreenshotsSendsOrderedList covers the relationship PATCH body.
func TestReorderAppScreenshotsSendsOrderedList(t *testing.T) {
	var got models.ResourceIdentifiers

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/v1/appScreenshotSets/set1/relationships/appScreenshots" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).ReorderAppScreenshots("set1", []string{"b", "a"}, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.Data) != 2 || got.Data[0].ID != "b" || got.Data[1].ID != "a" || got.Data[0].Type != "appScreenshots" {
		t.Errorf("body = %+v, want appScreenshots b then a", got.Data)
	}
}
