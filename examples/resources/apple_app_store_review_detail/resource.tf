data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_app_store_version" "v1" {
  app_id         = data.apple_apps.example.apps[0].id
  platform       = "IOS"
  version_string = "1.0"
}

# An app that works without signing in. Say so in the notes: a reviewer who
# cannot find the sign-in screen rejects the build.
resource "apple_app_store_review_detail" "v1" {
  app_store_version_id = apple_app_store_version.v1.id

  contact_first_name = "Ahmed"
  contact_last_name  = "Osman"
  contact_email      = "review@example.com"
  contact_phone      = "+20 109 255 8423"

  demo_account_required = false

  notes = <<-EOT
    No account and no sign-in are needed. Open the app and add a record by
    typing, by voice, or by photographing a receipt. The paywall is reached
    from Settings > Upgrade.
  EOT
}

# An app that does need one. demo_account_password is stored in Terraform state
# in plain text, so supply it from a secret store rather than writing it into
# the configuration, and use an account created for App Review alone.
resource "apple_app_store_review_detail" "with_demo_account" {
  app_store_version_id = apple_app_store_version.v1.id

  contact_first_name = "Ahmed"
  contact_last_name  = "Osman"
  contact_email      = "review@example.com"
  contact_phone      = "+20 109 255 8423"

  demo_account_required = true
  demo_account_name     = "appreview@example.com"
  demo_account_password = var.review_demo_password
}

variable "review_demo_password" {
  description = "Password for the App Review demo account. Stored in state in plain text."
  type        = string
  sensitive   = true
  default     = null
}
