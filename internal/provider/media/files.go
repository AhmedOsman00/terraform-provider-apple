// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

// Package media holds what the image-uploading resources share: reading a
// local file and checksumming it the way Apple does, and the one resource shape
// -- a single review screenshot under a product -- that subscriptions and
// in-app purchases both have.
//
// Files are named by path, and their content is tracked by checksum. The path
// is only ever an input: Apple keeps the bytes and their MD5, not where they
// came from. So a resource computes the checksum of the configured file at plan
// time and compares it with the one Apple reports, which is what makes editing
// an image in place show up as a diff, and what makes a screenshot replaced by
// hand in App Store Connect show up as drift.
package media

import (
	"fmt"
	"os"
	"strings"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple"
	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

// ReadAsset reads a local media file and returns its content and checksum.
func ReadAsset(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("could not read '%s': %w", path, err)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("'%s' is empty", path)
	}

	return data, apple.MediaChecksum(data), nil
}

// AssetChecksum is the checksum to record for an asset Apple reports.
//
// An asset Apple failed to process, or one it holds no checksum for, reports
// "", which can never equal a real file's checksum -- so the next plan replaces
// it rather than treating a broken image as the configured one.
func AssetChecksum(asset models.MediaAsset) string {
	if asset.DeliveryState() == models.AssetDeliveryFailed || asset.Attributes.SourceFileChecksum == nil {
		return ""
	}

	return strings.ToLower(*asset.Attributes.SourceFileChecksum)
}

// IsNotFound reports whether an API error means the record is gone.
func IsNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}
