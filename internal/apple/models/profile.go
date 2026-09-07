package models

import "time"

// Profile platform enumeration - same as Bundle ID platforms.
type ProfilePlatform string

const (
	ProfileIOS     ProfilePlatform = "IOS"
	ProfileMACOS   ProfilePlatform = "MAC_OS"
	ProfileTVOS    ProfilePlatform = "TV_OS"
	ProfileWATCHOS ProfilePlatform = "WATCH_OS"
)

// Profile type enumeration.
type ProfileType string

const (
	ProfileTypeDevelopment ProfileType = "IOS_APP_DEVELOPMENT"
	ProfileTypeAdhoc       ProfileType = "IOS_APP_ADHOC"
	ProfileTypeAppStore    ProfileType = "IOS_APP_STORE"
	ProfileTypeInHouse     ProfileType = "IOS_APP_INHOUSE"
	ProfileTypeMacDev      ProfileType = "MAC_APP_DEVELOPMENT"
	ProfileTypeMacStore    ProfileType = "MAC_APP_STORE"
	ProfileTypeMacDirect   ProfileType = "MAC_APP_DIRECT"
	ProfileTypeTVOSDev     ProfileType = "TVOS_APP_DEVELOPMENT"
	ProfileTypeTVOSAdhoc   ProfileType = "TVOS_APP_ADHOC"
	ProfileTypeTVOSStore   ProfileType = "TVOS_APP_STORE"
	ProfileTypeTVOSInHouse ProfileType = "TVOS_APP_INHOUSE"
)

// Profile state enumeration.
type ProfileState string

const (
	ProfileStateActive  ProfileState = "ACTIVE"
	ProfileStateInvalid ProfileState = "INVALID"
	ProfileStateExpired ProfileState = "EXPIRED"
)

// Profile resource model.
type Profile struct {
	Type          string            `json:"type"`
	ID            string            `json:"id"`
	Attributes    ProfileAttributes `json:"attributes"`
	Relationships *Relationships    `json:"relationships,omitempty"`
	Links         *ResourceLinks    `json:"links,omitempty"`
}

// Request models for Profile operations.
type ProfileCreateRequest struct {
	Type          string                     `json:"type"`
	Attributes    ProfileCreateAttributes    `json:"attributes"`
	Relationships ProfileCreateRelationships `json:"relationships"`
}

type ProfileUpdateRequest struct {
	Type       string                  `json:"type"`
	ID         string                  `json:"id"`
	Attributes ProfileUpdateAttributes `json:"attributes"`
}

// Profile attributes.
type ProfileAttributes struct {
	Name           string          `json:"name"`
	Platform       ProfilePlatform `json:"platform"`
	ProfileContent *string         `json:"profileContent,omitempty"` // Base64 encoded profile data
	UUID           *string         `json:"uuid,omitempty"`
	CreatedDate    *time.Time      `json:"createdDate,omitempty"`
	ProfileState   *ProfileState   `json:"profileState,omitempty"`
	ProfileType    *ProfileType    `json:"profileType,omitempty"`
	ExpirationDate *time.Time      `json:"expirationDate,omitempty"`
}

// Create attributes.
//
// Apple's POST /v1/profiles takes profileType, not platform: the type encodes
// the platform (IOS_APP_STORE implies IOS), and the response reports platform
// back as a computed attribute. Sending platform here instead of profileType
// is rejected for a missing required attribute.
type ProfileCreateAttributes struct {
	Name        string      `json:"name"`
	ProfileType ProfileType `json:"profileType"`
}

// Update attributes (only name can be updated for Profiles).
type ProfileUpdateAttributes struct {
	Name string `json:"name"`
}

// Profile relationships for creation.
type ProfileCreateRelationships struct {
	BundleID     ResourceIdentifier   `json:"bundleId"`
	Certificates ResourceIdentifiers  `json:"certificates"`
	Devices      *ResourceIdentifiers `json:"devices,omitempty"` // Omitted entirely for App Store profiles
}

// Resource identifier for to-one relationships.
type ResourceIdentifier struct {
	Data ResourceData `json:"data"`
}

// Resource identifiers for to-many relationships.
//
// JSON:API wraps a to-many relationship as a single object with an array under
// "data" -- {"certificates":{"data":[...]}} -- not as an array of to-one
// identifiers. Apple rejects the latter shape.
type ResourceIdentifiers struct {
	Data []ResourceData `json:"data"`
}

// Resource data for relationships.
type ResourceData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Profile relationships structure.
type Relationships struct {
	BundleID     *Relationship `json:"bundleId,omitempty"`
	Certificates *Relationship `json:"certificates,omitempty"`
	Devices      *Relationship `json:"devices,omitempty"`
}

// Individual relationship structure.
type Relationship struct {
	Links *RelationshipLinks `json:"links,omitempty"`
	Data  interface{}        `json:"data,omitempty"`
	Meta  *RelationshipMeta  `json:"meta,omitempty"`
}

// Relationship links.
type RelationshipLinks struct {
	Self    *string `json:"self,omitempty"`
	Related *string `json:"related,omitempty"`
}

// Relationship metadata.
type RelationshipMeta struct {
	Paging *PagingInformation `json:"paging,omitempty"`
}
