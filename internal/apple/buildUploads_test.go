// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package apple

import (
	"crypto/md5" //nolint:gosec // matches the checksum under test
	"encoding/hex"
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

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// fakeBuildUploads is the build upload API and the storage host its parts go
// to, on one TLS server.
type fakeBuildUploads struct {
	t        *testing.T
	srv      *httptest.Server
	failPart bool

	mu       sync.Mutex
	created  models.BuildUploadCreateAttributes
	reserved models.BuildUploadFileCreateAttributes
	parts    map[int64][]byte
	partAuth []string
	checksum models.Checksum
	deleted  []string
}

func newFakeBuildUploads(t *testing.T) *fakeBuildUploads {
	f := &fakeBuildUploads{t: t, parts: map[int64][]byte{}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeBuildUploads) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == "POST" && r.URL.Path == "/v1/buildUploads":
		var sent models.Request[models.BuildUploadCreateRequest]
		if err := json.Unmarshal(body, &sent); err != nil {
			f.t.Errorf("create body: %v", err)
		}
		if got := sent.Data.Relationships["app"].Data.ID; got != "APP1" {
			f.t.Errorf("app = %q, want APP1", got)
		}
		f.created = sent.Data.Attributes
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"data":{"type":"buildUploads","id":"U1","attributes":{"state":{"state":"AWAITING_UPLOAD"}}}}`)

	case r.Method == "POST" && r.URL.Path == "/v1/buildUploadFiles":
		var sent models.Request[models.BuildUploadFileCreateRequest]
		if err := json.Unmarshal(body, &sent); err != nil {
			f.t.Errorf("reservation body: %v", err)
		}
		if got := sent.Data.Relationships["buildUpload"].Data.ID; got != "U1" {
			f.t.Errorf("reservation parent = %q, want U1", got)
		}
		f.reserved = sent.Data.Attributes
		size, half := sent.Data.Attributes.FileSize, sent.Data.Attributes.FileSize/2
		ops := []models.UploadOperation{
			{Method: "PUT", URL: f.srv.URL + "/part/0", Offset: 0, Length: half},
			{Method: "PUT", URL: fmt.Sprintf("%s/part/%d", f.srv.URL, half), Offset: half, Length: size - half},
		}
		reply, _ := json.Marshal(models.Response[models.BuildUploadFile]{Data: models.BuildUploadFile{
			Type: "buildUploadFiles", ID: "F1", Attributes: models.BuildUploadFileAttributes{UploadOperations: ops},
		}})
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(reply)

	case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/part/"):
		f.partAuth = append(f.partAuth, r.Header.Get("Authorization"))
		if f.failPart {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var offset int64
		_, _ = fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/part/"), "%d", &offset)
		f.parts[offset] = body

	case r.Method == "PATCH" && r.URL.Path == "/v1/buildUploadFiles/F1":
		var sent models.Request[models.BuildUploadFileCommitRequest]
		if err := json.Unmarshal(body, &sent); err != nil {
			f.t.Errorf("commit body: %v", err)
		}
		if !sent.Data.Attributes.Uploaded {
			f.t.Error("the commit did not say uploaded")
		}
		f.checksum = sent.Data.Attributes.SourceFileChecksums.File
		fmt.Fprint(w, `{"data":{"type":"buildUploadFiles","id":"F1","attributes":{}}}`)

	case r.Method == "DELETE" && r.URL.Path == "/v1/buildUploads/U1":
		f.deleted = append(f.deleted, "U1")
		w.WriteHeader(http.StatusNoContent)

	default:
		f.t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeIPA(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "App.ipa")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUploadBuildDeliversEveryPartAndCommitsTheChecksum(t *testing.T) {
	t.Parallel()

	fake := newFakeBuildUploads(t)
	client := &Client{HostURL: fake.srv.URL, HTTPClient: fake.srv.Client(), UploadHTTPClient: fake.srv.Client(), Token: "test"}
	content := "not really an ipa, but bytes all the same"

	upload, err := client.UploadBuild("APP1", "IOS", "1.2.0", "7", writeIPA(t, content))
	if err != nil {
		t.Fatal(err)
	}
	if upload.ID != "U1" {
		t.Errorf("upload ID = %q", upload.ID)
	}

	want := models.BuildUploadCreateAttributes{CFBundleShortVersionString: "1.2.0", CFBundleVersion: "7", Platform: "IOS"}
	if fake.created != want {
		t.Errorf("created %+v, want %+v", fake.created, want)
	}
	if fake.reserved.FileName != "App.ipa" || fake.reserved.FileSize != int64(len(content)) ||
		fake.reserved.UTI != "com.apple.ipa" || fake.reserved.AssetType != "ASSET" {
		t.Errorf("reserved %+v", fake.reserved)
	}

	half := int64(len(content) / 2)
	if got := string(fake.parts[0]) + string(fake.parts[half]); got != content {
		t.Errorf("parts reassemble to %q", got)
	}
	for _, auth := range fake.partAuth {
		if auth != "" {
			t.Errorf("a part carried Authorization %q; parts must never carry the API token", auth)
		}
	}

	sum := md5.Sum([]byte(content)) //nolint:gosec // see import
	if fake.checksum.Hash != hex.EncodeToString(sum[:]) || fake.checksum.Algorithm != "MD5" {
		t.Errorf("committed checksum %+v", fake.checksum)
	}
	if len(fake.deleted) != 0 {
		t.Errorf("deleted %v after a successful upload", fake.deleted)
	}
}

func TestUploadBuildRemovesAnUploadItCouldNotDeliver(t *testing.T) {
	t.Parallel()

	fake := newFakeBuildUploads(t)
	fake.failPart = true
	client := &Client{HostURL: fake.srv.URL, HTTPClient: fake.srv.Client(), UploadHTTPClient: fake.srv.Client(), Token: "test"}

	if _, err := client.UploadBuild("APP1", "IOS", "1.2.0", "7", writeIPA(t, "some bytes")); err == nil {
		t.Fatal("expected an error when a part is refused")
	}
	if len(fake.deleted) != 1 {
		t.Errorf("deleted %v, want the upload removed", fake.deleted)
	}
}
