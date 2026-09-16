data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# An app can hold only one editable version per platform at a time; Apple
# answers a second with a 409 naming the one already being prepared. Import the
# existing one rather than creating another.
resource "apple_app_store_version" "v1" {
  app_id         = data.apple_apps.example.apps[0].id
  platform       = "IOS"
  version_string = "1.0"

  copyright = "2026 AO Studio"

  # AFTER_APPROVAL is "release automatically"; MANUAL holds the approved version
  # until it is released by hand.
  release_type = "MANUAL"
}

# A scheduled release needs a full ISO 8601 timestamp with seconds and a zone,
# not a plain date.
resource "apple_app_store_version" "v2" {
  app_id         = data.apple_apps.example.apps[0].id
  platform       = "IOS"
  version_string = "2.0"

  copyright             = "2026 AO Studio"
  release_type          = "SCHEDULED"
  earliest_release_date = "2026-03-01T08:00:00-07:00"
}
