# Users and Access.
# Removing an apple_user removes that person from the team, which the API cannot undo.

import {
  to = apple_user.dev_example_com
  id = "dev@example.com"
}
resource "apple_user" "dev_example_com" {
  username             = "dev@example.com"
  roles                = ["APP_MANAGER", "DEVELOPER"]
  all_apps_visible     = false
  provisioning_allowed = true
  visible_apps         = ["6478123456"]
}

import {
  to = apple_user_invitation.new_example_com
  id = "new@example.com"
}
resource "apple_user_invitation" "new_example_com" {
  email            = "new@example.com"
  first_name       = "New"
  last_name        = "Hire"
  roles            = ["MARKETING"]
  all_apps_visible = true
}

