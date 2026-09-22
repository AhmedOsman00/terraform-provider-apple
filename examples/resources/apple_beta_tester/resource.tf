data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_beta_group" "qa" {
  app_id                   = data.apple_apps.example.apps[0].id
  name                     = "QA"
  is_internal_group        = true
  has_access_to_all_builds = true
}

# An internal group's testers must be users of your App Store Connect team —
# Apple rejects any other address. Add them under Users and Access first.
resource "apple_beta_tester" "qa" {
  for_each = toset([
    "qa.lead@example.com",
    "release.manager@example.com",
  ])

  group_id = apple_beta_group.qa.id
  email    = each.value
}

resource "apple_beta_group" "early_access" {
  app_id           = data.apple_apps.example.apps[0].id
  name             = "Early access"
  feedback_enabled = true
}

# An external group takes any address, up to 10,000 per app — but nothing
# reaches them until beta review has approved a build. See
# apple_beta_app_review_detail for what the reviewers are told.
#
# Applying this sends each address a real TestFlight invitation, so name only
# people you are entitled to invite.
resource "apple_beta_tester" "early_access" {
  for_each = {
    "ada@example.com"   = { first_name = "Ada", last_name = "Lovelace" }
    "grace@example.com" = { first_name = "Grace", last_name = "Hopper" }
  }

  group_id = apple_beta_group.early_access.id
  email    = each.key

  # Sent only when the provider creates the tester record. Apple publishes no
  # update for one, so an address already in your account keeps the name it has.
  first_name = each.value.first_name
  last_name  = each.value.last_name
}

# A tester record belongs to the account rather than to one group, so putting
# the same person in a second group is a second resource sharing one Apple ID.
# No depends_on is needed: whichever of the two applies second finds the record
# already there and links it to its own group.
resource "apple_beta_tester" "ada_in_qa" {
  group_id = apple_beta_group.qa.id
  email    = "ada@example.com"
}
