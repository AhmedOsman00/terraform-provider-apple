# This adopts somebody who is already on the team; it cannot add one. A person
# joins by accepting an apple_user_invitation, and this takes over from there.
#
# Destroying it removes them from the team and revokes their access to every app.
resource "apple_user" "developer" {
  username = "ada@example.com"
  roles    = ["DEVELOPER"]

  # The access apple_certificate and apple_profile need.
  provisioning_allowed = true
  all_apps_visible     = true
}

data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# A member restricted to a named set of apps. Leave visible_apps unset to leave
# Apple's existing answer alone: it is only read back when the configuration
# sets it, because a member who sees everything is reported as seeing every app
# on the team.
resource "apple_user" "contractor" {
  username = "contractor@example.com"
  roles    = ["APP_MANAGER"]

  all_apps_visible = false
  visible_apps     = [data.apple_apps.example.apps[0].id]
}

# Roles are a set, so order is not a diff. ADMIN subsumes the rest; naming
# another role beside it changes nothing. ACCOUNT_HOLDER is rejected — Apple
# reports it for the person who owns the membership and refuses to grant it.
resource "apple_user" "release_manager" {
  username = "grace@example.com"
  roles = [
    "APP_MANAGER",
    "ACCESS_TO_REPORTS",
    "CUSTOMER_SUPPORT",
  ]
}
