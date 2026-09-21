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

# The half of TestFlight metadata that is replaced with every upload. The build
# is named by the two numbers a pipeline already exports, not by Apple's opaque
# build ID — the provider resolves that itself.
#
# This provider does not upload builds: the binary must already be in App Store
# Connect, put there by Xcode, Transporter or fastlane, and Apple leaves it
# PROCESSING for five to thirty minutes afterwards.
resource "apple_beta_build_localization" "whats_new" {
  for_each = {
    "en-US" = "Shared budgets, and a rewritten sync engine. Please add a budget on one device and check that it appears on another within a few seconds."
    "ar-SA" = "الميزانيات المشتركة ومحرك مزامنة جديد. أضف ميزانية على جهاز وتأكد من ظهورها على جهاز آخر خلال ثوانٍ."
  }

  app_id              = data.apple_apps.example.apps[0].id
  build_number        = var.build_number
  pre_release_version = var.marketing_version
  locale              = each.key
  whats_new           = each.value
}
