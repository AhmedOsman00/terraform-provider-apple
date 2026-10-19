data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

variable "build_number" {
  description = "CFBundleVersion of the build already uploaded to App Store Connect (CURRENT_PROJECT_VERSION)."
  type        = string
  default     = "42"
}

variable "marketing_version" {
  description = "CFBundleShortVersionString of the build's train (MARKETING_VERSION)."
  type        = string
  default     = "1.4.0"
}

# A group that chooses its builds rather than receiving every one.
resource "apple_beta_group" "release_candidates" {
  app_id                   = data.apple_apps.example.apps[0].id
  name                     = "Release candidates"
  is_internal_group        = true
  has_access_to_all_builds = false
}

# Gives the group the build a pipeline just uploaded and waited for:
#
#   apple build upload build/App.ipa
#   apple build wait "$version" "$build"
#   TF_VAR_build_number="$build" TF_VAR_marketing_version="$version" terraform apply
#
# Changing build_number replaces the assignment: the previous build comes off
# the group and the new one goes on. Apple refuses a build that is still
# processing.
resource "apple_beta_group_build" "release_candidate" {
  group_id            = apple_beta_group.release_candidates.id
  build_number        = var.build_number
  pre_release_version = var.marketing_version
}

# An external group's testers see an assigned build only after Apple's beta
# review approves it, and submitting it for review stays in App Store Connect.
resource "apple_beta_group" "early_access" {
  app_id = data.apple_apps.example.apps[0].id
  name   = "Early access"
}

resource "apple_beta_group_build" "early_access" {
  group_id            = apple_beta_group.early_access.id
  build_number        = var.build_number
  pre_release_version = var.marketing_version
}
