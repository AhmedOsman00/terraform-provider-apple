// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package media

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

func TestReadAsset(t *testing.T) {
	dir := t.TempDir()

	file := filepath.Join(dir, "a.png")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, checksum, err := ReadAsset(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("data = %q", data)
	}
	// md5("hello")
	if checksum != "5d41402abc4b2a76b9719d911017c592" {
		t.Errorf("checksum = %q", checksum)
	}

	empty := filepath.Join(dir, "empty.png")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadAsset(empty); err == nil {
		t.Error("expected an error for an empty file")
	}
	if _, _, err := ReadAsset(filepath.Join(dir, "missing.png")); err == nil {
		t.Error("expected an error for a missing file")
	}
}

// TestAssetChecksum covers the two cases that must never match a real file:
// an asset Apple failed to process and one with no checksum.
func TestAssetChecksum(t *testing.T) {
	upper := "5D41402ABC4B2A76B9719D911017C592"

	cases := map[string]struct {
		asset models.MediaAsset
		want  string
	}{
		"complete, normalised to lower case": {
			asset: models.MediaAsset{Attributes: models.MediaAssetAttributes{
				SourceFileChecksum: &upper,
				AssetDeliveryState: &models.AppMediaAssetState{State: models.AssetDeliveryComplete},
			}},
			want: "5d41402abc4b2a76b9719d911017c592",
		},
		"failed": {
			asset: models.MediaAsset{Attributes: models.MediaAssetAttributes{
				SourceFileChecksum: &upper,
				AssetDeliveryState: &models.AppMediaAssetState{State: models.AssetDeliveryFailed},
			}},
			want: "",
		},
		"no checksum": {
			asset: models.MediaAsset{},
			want:  "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := AssetChecksum(tc.asset); got != tc.want {
				t.Errorf("AssetChecksum = %q, want %q", got, tc.want)
			}
		})
	}
}
