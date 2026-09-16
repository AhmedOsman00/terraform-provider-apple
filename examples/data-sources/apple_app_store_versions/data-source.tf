data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# An app accumulates a version record per release for its whole life, one per
# platform, so platform is usually worth setting -- it is passed to Apple
# rather than applied in memory.
data "apple_app_store_versions" "all" {
  app_id   = data.apple_apps.example.apps[0].id
  platform = "IOS"
}

# The common use: find the version currently being prepared, so a localization
# or review detail can be attached to a release this configuration did not
# create.
data "apple_app_store_versions" "editable" {
  app_id            = data.apple_apps.example.apps[0].id
  platform          = "IOS"
  app_version_state = "PREPARE_FOR_SUBMISSION"
}

# The most recent release. Sort by created_date rather than version_string:
# version strings are compared as text, so "1.10" sorts before "1.9".
data "apple_app_store_versions" "latest" {
  app_id     = data.apple_apps.example.apps[0].id
  platform   = "IOS"
  sort_by    = "created_date"
  sort_order = "desc"
  limit      = 1
}

output "editable_version_id" {
  value = try(data.apple_app_store_versions.editable.versions[0].id, null)
}

output "latest_version_string" {
  value = try(data.apple_app_store_versions.latest.versions[0].version_string, null)
}
