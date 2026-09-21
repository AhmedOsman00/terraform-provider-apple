data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# What Apple's beta reviewers are told before they test a build bound for
# external testers. A different record from apple_app_store_review_detail: this
# one is per app and gates external TestFlight distribution, that one is per
# version and gates the App Store listing.
#
# An app that only ever distributes to internal groups does not need this at
# all — internal testers skip beta review.
resource "apple_beta_app_review_detail" "example" {
  app_id = data.apple_apps.example.apps[0].id

  contact_first_name = "Ahmed"
  contact_last_name  = "Osman"
  contact_email      = "review@example.com"
  contact_phone      = "+20 109 255 8423"

  demo_account_required = true
  demo_account_name     = "betareview@example.com"

  # Stored in Terraform state in plain text, so supply it from a secret store
  # and use an account created for review alone.
  demo_account_password = var.beta_review_demo_password

  notes = <<-EOT
    Sign in with the demo account above. The shared budgets feature under test
    is reached from the Budgets tab; the paywall is reached from
    Settings > Upgrade and is disabled for this account.
  EOT
}

variable "beta_review_demo_password" {
  description = "Password for the beta review demo account. Stored in state in plain text."
  type        = string
  sensitive   = true
  default     = null
}
