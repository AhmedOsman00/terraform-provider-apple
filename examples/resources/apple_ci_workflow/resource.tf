variable "product_id" {
  type        = string
  description = "Existing Xcode Cloud product ID. Set up the product in Xcode first."
}
variable "repository_id" {
  type        = string
  description = "Existing connected SCM repository ID."
}
variable "xcode_version_id" {
  type        = string
  description = "Xcode version ID from apple_ci_xcode_versions."
}
variable "mac_os_version_id" {
  type        = string
  description = "Compatible macOS version ID from apple_ci_mac_os_versions."
}

resource "apple_ci_workflow" "main" {
  name                = "Main"
  description         = "Build and test main"
  is_enabled          = false
  clean               = false
  container_file_path = "App.xcodeproj"
  product_id          = var.product_id
  repository_id       = var.repository_id
  xcode_version_id    = var.xcode_version_id
  mac_os_version_id   = var.mac_os_version_id

  branch_start_condition = {
    source      = { patterns = [{ pattern = "main", is_prefix = false }] }
    auto_cancel = true
    files_and_folders_rule = {
      mode     = "START_IF_ANY_FILE_MATCHES"
      matchers = [{ file_extension = "swift" }]
    }
  }
  tag_start_condition = {
    source = { patterns = [{ pattern = "v", is_prefix = true }] }
  }
  pull_request_start_condition = {
    source      = { is_all_match = true }
    destination = { patterns = [{ pattern = "main", is_prefix = false }] }
    auto_cancel = true
  }
  scheduled_start_condition = {
    source = { patterns = [{ pattern = "main", is_prefix = false }] }
    schedule = {
      frequency = "WEEKLY"
      days      = ["MONDAY"]
      hour      = 9
      minute    = 0
      timezone  = "Europe/Warsaw"
    }
  }
  manual_branch_start_condition = { source = { is_all_match = true } }
  manual_tag_start_condition    = { source = { is_all_match = true } }
  manual_pull_request_start_condition = {
    source      = { is_all_match = true }
    destination = { is_all_match = true }
  }
  actions = [
    {
      action_type         = "BUILD"
      scheme              = "App"
      platform            = "IOS"
      destination         = "ANY_IOS_DEVICE"
      is_required_to_pass = true
    },
    {
      action_type         = "TEST"
      scheme              = "App"
      platform            = "IOS"
      is_required_to_pass = true
      test_configuration = {
        kind = "USE_SCHEME_SETTINGS"
        test_destinations = [{
          kind                   = "SIMULATOR"
          device_type_identifier = "com.apple.CoreSimulator.SimDeviceType.iPhone-14-Pro"
          runtime_identifier     = "default"
        }]
      }
    },
    {
      action_type         = "ANALYZE"
      scheme              = "App"
      platform            = "IOS"
      destination         = "ANY_IOS_DEVICE"
      is_required_to_pass = true
    },
    {
      action_type                 = "ARCHIVE"
      scheme                      = "App"
      platform                    = "IOS"
      destination                 = "ANY_IOS_DEVICE"
      is_required_to_pass         = true
      build_distribution_audience = "APP_STORE_ELIGIBLE"
    }
  ]
}

# Configure post-actions and environment variables/secrets in App Store Connect.
# Archive eligibility above does not configure TestFlight distribution.
