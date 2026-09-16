data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# The app's name, subtitle and privacy policy link survive every release, which
# is why they live here rather than on a version. The description, keywords and
# promotional text belong to one release and live on
# apple_app_store_version_localization.
#
# name and subtitle are capped at 30 characters each, counted in characters
# rather than bytes -- an Arabic or Japanese name gets the full thirty.
resource "apple_app_info_localization" "en" {
  app_id = data.apple_apps.example.apps[0].id
  locale = "en-US"

  name               = "Fenn"
  subtitle           = "Private money tracking"
  privacy_policy_url = "https://example.com/privacy"
}

resource "apple_app_info_localization" "ar" {
  app_id = data.apple_apps.example.apps[0].id
  locale = "ar-SA"

  name               = "Fenn"
  subtitle           = "تتبّع مصروفاتك بخصوصية"
  privacy_policy_url = "https://example.com/ar/privacy"
}

# Note that privacy_policy_url is a link on the product page. It is NOT App
# Store Connect's App Privacy section -- the data-collection questionnaire that
# blocks a submission with "Admin must provide information about the app's
# privacy practices". Apple publishes no API for that at all; it has to be
# answered once on the App Store Connect website.
