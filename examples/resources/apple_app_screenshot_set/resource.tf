data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_app_store_version" "v1" {
  app_id         = data.apple_apps.example.apps[0].id
  platform       = "IOS"
  version_string = "1.0"
}

resource "apple_app_store_version_localization" "en" {
  app_store_version_id = apple_app_store_version.v1.id
  locale               = "en-US"
  description          = "Track every expense in seconds."
}

# One set per device size. The list is the display order on the App Store;
# reordering it reorders the screenshots without uploading anything, and only a
# file whose content changed is uploaded again.
resource "apple_app_screenshot_set" "iphone_67" {
  app_store_version_localization_id = apple_app_store_version_localization.en.id
  screenshot_display_type           = "APP_IPHONE_67"

  screenshots = [
    "${path.module}/screenshots/en-US/iphone67/01-home.png",
    "${path.module}/screenshots/en-US/iphone67/02-add-expense.png",
    "${path.module}/screenshots/en-US/iphone67/03-reports.png",
  ]
}

# Another device size is another set. With a directory per locale and device,
# the list can come from fileset() instead -- it returns paths sorted, so
# prefixing files with their position fixes the order:
#
#   screenshots = [for f in sort(fileset(local.ipad_dir, "*.png")) : "${local.ipad_dir}/${f}"]
resource "apple_app_screenshot_set" "ipad_13" {
  app_store_version_localization_id = apple_app_store_version_localization.en.id
  screenshot_display_type           = "APP_IPAD_PRO_3GEN_129"

  screenshots = [
    "${path.module}/screenshots/en-US/ipad13/01-home.png",
    "${path.module}/screenshots/en-US/ipad13/02-reports.png",
  ]
}
