// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"slices"
	"sort"

	"github.com/AhmedOsman00/terraform-provider-apple/internal/apple/models"
)

const accountHolder = "ACCOUNT_HOLDER"

// exportTeam writes Users and Access: the members of the team and the
// invitations still waiting for an answer.
func (e *exporter) exportTeam() error {
	f := e.file("team.tf")
	f.comment("Users and Access.\nRemoving an apple_user removes that person from the team, which the API cannot undo.")

	e.progress("Reading team members")
	users, err := e.src.GetUsers()
	if err != nil {
		if err := e.failed("the team's members (this needs an API key with the Admin role)", err); err != nil {
			return err
		}
	}
	for _, user := range users {
		if err := e.exportUser(f, user); err != nil {
			return err
		}
	}

	e.progress("Reading invitations")
	invitations, err := e.src.GetUserInvitations()
	if err != nil {
		return e.failed("the team's invitations", err)
	}
	for _, invitation := range invitations {
		if err := e.exportInvitation(f, invitation); err != nil {
			return err
		}
	}

	return nil
}

func (e *exporter) exportUser(f *file, user models.User) error {
	attributes := user.Attributes
	if attributes.Username == nil {
		return nil
	}
	username := *attributes.Username

	// Apple refuses every write that grants ACCOUNT_HOLDER, and the provider
	// refuses it at plan time, so the account holder cannot be managed.
	if slices.Contains(attributes.Roles, accountHolder) {
		e.note(Skipped, "", "%s is the Account Holder, which the API cannot manage, so they were left out.", username)
		return nil
	}

	b, addr := e.resource(f, "apple_user", user.ID, username, username)
	b.str("username", username)
	b.strings("roles", sorted(attributes.Roles))
	b.optBool("all_apps_visible", attributes.AllAppsVisible)
	b.optBool("provisioning_allowed", attributes.ProvisioningAllowed)

	// visible_apps is read only for a member restricted to some apps, as the
	// import does: for everybody else Apple answers with every app.
	if attributes.AllAppsVisible != nil && !*attributes.AllAppsVisible {
		apps, err := e.src.GetUserVisibleApps(user.ID)
		if err != nil {
			return e.failed("the apps "+username+" can see", err)
		}
		b.strings("visible_apps", sorted(appIDs(apps)))
	}

	if slices.Contains(attributes.Roles, "ADMIN") {
		e.note(Attention, addr.String(), "%s is an Admin. An Admin can change every other member, including through this configuration's API key.", username)
	}
	return nil
}

func (e *exporter) exportInvitation(f *file, invitation models.UserInvitation) error {
	attributes := invitation.Attributes
	if attributes.Email == nil || attributes.FirstName == nil || attributes.LastName == nil {
		return nil
	}
	email := *attributes.Email

	b, addr := e.resource(f, "apple_user_invitation", invitation.ID, email, email)
	b.str("email", email)
	b.str("first_name", *attributes.FirstName)
	b.str("last_name", *attributes.LastName)
	b.strings("roles", sorted(attributes.Roles))
	b.optBool("all_apps_visible", attributes.AllAppsVisible)
	b.optBool("provisioning_allowed", attributes.ProvisioningAllowed)

	if attributes.AllAppsVisible != nil && !*attributes.AllAppsVisible {
		apps, err := e.src.GetUserInvitationVisibleApps(invitation.ID)
		if err != nil {
			return e.failed("the apps the invitation to "+email+" grants", err)
		}
		b.strings("visible_apps", sorted(appIDs(apps)))
	}

	expires := "soon"
	if attributes.ExpirationDate != nil {
		expires = "at " + *attributes.ExpirationDate
	}
	e.note(Attention, addr.String(),
		"The invitation to %s is pending and lapses %s. Once it is accepted Apple deletes it; replace this block with an apple_user for them then.", email, expires)
	return nil
}

func appIDs(apps []models.App) []string {
	ids := make([]string, 0, len(apps))
	for _, app := range apps {
		ids = append(ids, app.ID)
	}
	return ids
}

// sorted orders the elements of a set, so output is stable.
func sorted(values []string) []string {
	ordered := slices.Clone(values)
	sort.Strings(ordered)
	return ordered
}
