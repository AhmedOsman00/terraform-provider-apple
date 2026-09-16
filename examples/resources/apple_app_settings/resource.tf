data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# This resource adopts an app record that already exists -- Apple's API cannot
# create one -- and patches the account-level settings on it. terraform destroy
# drops it from state and warns; nothing at Apple changes.
#
# content_rights_declaration is the App Information question App Store Connect
# blocks a submission on: "You must set up Content Rights Information in App
# Information". Leaving it unset leaves Apple's answer alone, which for a new
# app means unanswered.
resource "apple_app_settings" "example" {
  app_id = data.apple_apps.example.apps[0].id

  content_rights_declaration = "DOES_NOT_USE_THIRD_PARTY_CONTENT"
  primary_locale             = "en-US"
}

# An app selling auto-renewable subscriptions usually wants the server
# notification endpoints set here too.
resource "apple_app_settings" "with_notifications" {
  app_id = data.apple_apps.example.apps[0].id

  content_rights_declaration      = "USES_THIRD_PARTY_CONTENT"
  primary_locale                  = "en-US"
  subscription_status_url         = "https://example.com/appstore/notifications"
  subscription_status_url_version = "V2"
}
