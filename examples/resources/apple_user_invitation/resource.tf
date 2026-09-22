# Inviting somebody is the only way to add a member: Apple publishes no endpoint
# that creates one directly. Applying this sends a real invitation email, so name
# only addresses you are entitled to invite.
resource "apple_user_invitation" "developer" {
  email      = "ada@example.com"
  first_name = "Ada"
  last_name  = "Lovelace"
  roles      = ["DEVELOPER"]

  # Developers who build locally need this to create signing certificates and
  # provisioning profiles of their own.
  provisioning_allowed = true

  # Leave all_apps_visible unset to let Apple decide, or say which it is.
  all_apps_visible = true
}

# A contractor who should see one app and nothing else.
data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_user_invitation" "contractor" {
  email      = "contractor@example.com"
  first_name = "Grace"
  last_name  = "Hopper"
  roles      = ["APP_MANAGER"]

  all_apps_visible = false
  visible_apps     = [data.apple_apps.example.apps[0].id]
}

# Apple destroys an invitation the moment it is accepted and issues a team member
# in its place, so this is how a configuration finds out that somebody joined.
output "developer_joined" {
  value = apple_user_invitation.developer.accepted
}

# Invitations lapse after 72 hours.
output "developer_invitation_expires" {
  value = apple_user_invitation.developer.expiration_date
}
