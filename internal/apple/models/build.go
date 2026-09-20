// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// Processing state enumeration for an uploaded build.
//
// A build is PROCESSING for five to thirty minutes after Xcode, Transporter or
// fastlane finishes uploading it, and Apple refuses to attach one to a version
// until it reaches VALID.
const (
	BuildProcessingStateProcessing = "PROCESSING"
	BuildProcessingStateFailed     = "FAILED"
	BuildProcessingStateInvalid    = "INVALID"
	BuildProcessingStateValid      = "VALID"
)

// Build is one uploaded binary.
//
// Apple identifies a build by two numbers, neither of which is its ID: the
// build number (CFBundleVersion, reported here as "version") and the marketing
// version of the train it belongs to (CFBundleShortVersionString, which lives
// on the pre-release version rather than on the build).
type Build struct {
	Type          string              `json:"type"`
	ID            string              `json:"id"`
	Attributes    BuildAttributes     `json:"attributes"`
	Relationships *BuildRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks      `json:"links,omitempty"`
}

// BuildAttributes reports what Apple knows about the binary.
//
// "version" is the build number, not the marketing version -- the name is
// Apple's and is the opposite way round from every other version field in this
// package.
type BuildAttributes struct {
	Version                 *string `json:"version,omitempty"`
	UploadedDate            *string `json:"uploadedDate,omitempty"`
	ExpirationDate          *string `json:"expirationDate,omitempty"`
	Expired                 *bool   `json:"expired,omitempty"`
	MinOsVersion            *string `json:"minOsVersion,omitempty"`
	ProcessingState         *string `json:"processingState,omitempty"`
	BuildAudienceType       *string `json:"buildAudienceType,omitempty"`
	UsesNonExemptEncryption *bool   `json:"usesNonExemptEncryption,omitempty"`
}

type BuildRelationships struct {
	App               *ResourceIdentifier `json:"app,omitempty"`
	PreReleaseVersion *ResourceIdentifier `json:"preReleaseVersion,omitempty"`
	AppStoreVersion   *ResourceIdentifier `json:"appStoreVersion,omitempty"`
}

// AppStoreVersionBuildLinkageRequest attaches a build to a version, or detaches
// the one attached.
//
// Apple publishes no way to set the build when the version is created --
// AppStoreVersionCreateRequest carries only the app relationship -- so this is
// always a second call. It uses NullableResourceIdentifier for the same reason
// AppInfoUpdateRelationships does: detaching is an explicit {"data":null}, and
// ResourceIdentifier would marshal an unset value as {"type":"","id":""}.
//
// The endpoint answers 204 with no body, so there is nothing to decode.
type AppStoreVersionBuildLinkageRequest = NullableResourceIdentifier
