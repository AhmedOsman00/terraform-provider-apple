// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package models

// User is one member of the App Store Connect team.
//
// The record cannot be created: a person joins the team by accepting a
// UserInvitation, and the invitation becomes this the moment they do. It can be
// updated and it can be deleted -- DELETE /v1/users/{id} removes the person
// from the team entirely -- which is the opposite shape from the records this
// provider adopts and never deletes.
//
// Apple publishes no GET for a single user either: the collection is the only
// way to read one, which is why the client resolves a user by scanning
// /v1/users rather than addressing it by ID.
type User struct {
	Type          string             `json:"type"`
	ID            string             `json:"id"`
	Attributes    UserAttributes     `json:"attributes"`
	Relationships *UserRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks     `json:"links,omitempty"`
}

// UserAttributes reports what Apple holds about a team member.
//
// username is the Apple Account address the person signs in with, and is the
// only stable human-readable handle: firstName and lastName are the person's
// own and Apple's update request carries neither.
type UserAttributes struct {
	Username            *string  `json:"username,omitempty"`
	FirstName           *string  `json:"firstName,omitempty"`
	LastName            *string  `json:"lastName,omitempty"`
	Roles               []string `json:"roles,omitempty"`
	AllAppsVisible      *bool    `json:"allAppsVisible,omitempty"`
	ProvisioningAllowed *bool    `json:"provisioningAllowed,omitempty"`
}

type UserRelationships struct {
	VisibleApps *ResourceIdentifiers `json:"visibleApps,omitempty"`
}

type UserUpdateRequest struct {
	Type          string                   `json:"type"`
	ID            string                   `json:"id"`
	Attributes    UserUpdateAttributes     `json:"attributes"`
	Relationships *UserUpdateRelationships `json:"relationships,omitempty"`
}

// UserUpdateAttributes is everything Apple lets a team change about a member.
//
// The name and the username are absent: they belong to the person's Apple
// Account rather than to the team's record of them.
type UserUpdateAttributes struct {
	Roles               []string `json:"roles,omitempty"`
	AllAppsVisible      *bool    `json:"allAppsVisible,omitempty"`
	ProvisioningAllowed *bool    `json:"provisioningAllowed,omitempty"`
}

// UserUpdateRelationships replaces the set of apps a user can see.
//
// The member is omitted entirely when allAppsVisible is true -- a user who sees
// everything has no list -- and sent as an empty array to clear one, which is
// why Data is a slice rather than a pointer: an explicit [] is meaningful here
// where an omitted member is not.
type UserUpdateRelationships struct {
	VisibleApps ResourceIdentifiers `json:"visibleApps"`
}

// UserInvitation is a pending invitation to join the team.
//
// It is a different record from User with a different lifetime: Apple creates
// it on POST, emails the address, and **destroys it when the person accepts**,
// issuing a User in its place. So an invitation that can no longer be read has
// either expired, been cancelled, or succeeded, and only the team's user list
// can say which.
type UserInvitation struct {
	Type          string                       `json:"type"`
	ID            string                       `json:"id"`
	Attributes    UserInvitationAttributes     `json:"attributes"`
	Relationships *UserInvitationRelationships `json:"relationships,omitempty"`
	Links         *ResourceLinks               `json:"links,omitempty"`
}

// UserInvitationAttributes reports what Apple holds about a pending invitation.
//
// expirationDate is Apple's: an invitation lapses after 72 hours and the person
// has to be invited again.
type UserInvitationAttributes struct {
	Email               *string  `json:"email,omitempty"`
	FirstName           *string  `json:"firstName,omitempty"`
	LastName            *string  `json:"lastName,omitempty"`
	Roles               []string `json:"roles,omitempty"`
	AllAppsVisible      *bool    `json:"allAppsVisible,omitempty"`
	ProvisioningAllowed *bool    `json:"provisioningAllowed,omitempty"`
	ExpirationDate      *string  `json:"expirationDate,omitempty"`
}

type UserInvitationRelationships struct {
	VisibleApps *ResourceIdentifiers `json:"visibleApps,omitempty"`
}

type UserInvitationCreateRequest struct {
	Type          string                             `json:"type"`
	Attributes    UserInvitationCreateAttributes     `json:"attributes"`
	Relationships *UserInvitationCreateRelationships `json:"relationships,omitempty"`
}

// UserInvitationCreateAttributes is the whole of what an invitation can say.
// Apple publishes no PATCH for one, so this is also the only place any of it
// can be set: changing an answer means cancelling the invitation and sending
// another.
type UserInvitationCreateAttributes struct {
	Email               string   `json:"email"`
	FirstName           string   `json:"firstName"`
	LastName            string   `json:"lastName"`
	Roles               []string `json:"roles"`
	AllAppsVisible      *bool    `json:"allAppsVisible,omitempty"`
	ProvisioningAllowed *bool    `json:"provisioningAllowed,omitempty"`
}

type UserInvitationCreateRelationships struct {
	VisibleApps ResourceIdentifiers `json:"visibleApps"`
}

// App Store Connect user roles.
//
// These are the permissions App Store Connect shows under Users and Access.
// Two of them are not ordinary answers:
//
//   - ACCOUNT_HOLDER is reported for the person who owns the Apple Developer
//     Program membership and cannot be assigned to anybody else. Apple rejects
//     a request that tries.
//   - ADMIN subsumes the rest; sending it alongside another role is accepted
//     but says nothing extra.
const (
	UserRoleAdmin                       = "ADMIN"
	UserRoleFinance                     = "FINANCE"
	UserRoleAccountHolder               = "ACCOUNT_HOLDER"
	UserRoleSales                       = "SALES"
	UserRoleMarketing                   = "MARKETING"
	UserRoleAppManager                  = "APP_MANAGER"
	UserRoleDeveloper                   = "DEVELOPER"
	UserRoleAccessToReports             = "ACCESS_TO_REPORTS"
	UserRoleCustomerSupport             = "CUSTOMER_SUPPORT"
	UserRoleCreateApps                  = "CREATE_APPS"
	UserRoleCloudManagedDeveloperID     = "CLOUD_MANAGED_DEVELOPER_ID"
	UserRoleCloudManagedAppDistribution = "CLOUD_MANAGED_APP_DISTRIBUTION"
	UserRoleGenerateIndividualKeys      = "GENERATE_INDIVIDUAL_KEYS"
)

// ValidUserRoles lists every role App Store Connect reports.
//
// ACCOUNT_HOLDER is included because Apple reports it and a configuration has
// to be able to hold what it reads; it is not assignable.
var ValidUserRoles = []string{
	UserRoleAdmin,
	UserRoleFinance,
	UserRoleAccountHolder,
	UserRoleSales,
	UserRoleMarketing,
	UserRoleAppManager,
	UserRoleDeveloper,
	UserRoleAccessToReports,
	UserRoleCustomerSupport,
	UserRoleCreateApps,
	UserRoleCloudManagedDeveloperID,
	UserRoleCloudManagedAppDistribution,
	UserRoleGenerateIndividualKeys,
}
