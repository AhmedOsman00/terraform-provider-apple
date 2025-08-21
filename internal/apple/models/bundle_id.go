package models

// Bundle ID platform enumeration
type BundleIDPlatform string

const (
	IOS     BundleIDPlatform = "IOS"
	MACOS   BundleIDPlatform = "MAC_OS"
	TVOS    BundleIDPlatform = "TV_OS"
	WATCHOS BundleIDPlatform = "WATCH_OS"
)

// Bundle ID resource model
type BundleID struct {
	Type       string             `json:"type"`
	ID         string             `json:"id"`
	Attributes BundleIDAttributes `json:"attributes"`
	Links      *ResourceLinks     `json:"links,omitempty"`
}

// Request models for Bundle ID operations
type BundleIDCreateRequest struct {
	Type       string             `json:"type"`
	Attributes BundleIDAttributes `json:"attributes"`
}

type BundleIDUpdateRequest struct {
	Type       string                   `json:"type"`
	ID         string                   `json:"id"`
	Attributes BundleIDUpdateAttributes `json:"attributes"`
}

// Bundle ID attributes
type BundleIDAttributes struct {
	Identifier string           `json:"identifier"`
	Name       string           `json:"name"`
	Platform   BundleIDPlatform `json:"platform"`
	SeedID     string           `json:"seedId,omitempty"`
}

// Update attributes (only name can be updated for Bundle IDs)
type BundleIDUpdateAttributes struct {
	Name string `json:"name"`
}
