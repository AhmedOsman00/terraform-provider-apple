data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# An internal group: members are users of your App Store Connect team, capped at
# 100, and a build reaches them as soon as it finishes processing. No beta
# review, and no public link — Apple rejects the public link attributes here.
resource "apple_beta_group" "qa" {
  app_id                   = data.apple_apps.example.apps[0].id
  name                     = "QA"
  is_internal_group        = true
  has_access_to_all_builds = true
  feedback_enabled         = true
}

# An external group: members are arbitrary email addresses, up to 10,000 per
# app, and a build cannot reach them until Apple's beta review has approved one.
# See apple_beta_app_review_detail for what the beta reviewers are told.
resource "apple_beta_group" "public_beta" {
  app_id           = data.apple_apps.example.apps[0].id
  name             = "Public beta"
  feedback_enabled = true

  # A public link anyone can join through, closed once 500 testers have taken
  # it. Closing it entirely is public_link_enabled = false; a limit of zero is
  # rejected rather than treated as closed.
  public_link_enabled       = true
  public_link_limit_enabled = true
  public_link_limit         = 500
}

output "public_beta_link" {
  description = "The TestFlight URL Apple issued for the public beta group."
  value       = apple_beta_group.public_beta.public_link
}
