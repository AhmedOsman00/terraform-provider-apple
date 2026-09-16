data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# List the valid category constants, and which platforms accept them, with the
# apple_app_categories data source. Only a top-level category can be a primary
# or secondary category.
data "apple_app_categories" "top_level" {
  platforms      = ["IOS"]
  top_level_only = true
}

# Categories hang off the app's AppInfo record, which Apple creates with the app
# and freezes once a version is in review. This resource adopts the editable
# record rather than creating one, and terraform destroy drops it from state --
# clearing the categories instead would leave the app unsubmittable.
#
# A category removed from the configuration is cleared at Apple: every unset
# member is sent as an explicit null.
resource "apple_app_info" "example" {
  app_id = data.apple_apps.example.apps[0].id

  primary_category   = "FINANCE"
  secondary_category = "PRODUCTIVITY"
}

# Some categories -- GAMES and STICKERS among them -- carry subcategories. Apple
# rejects a subcategory that does not belong to the category it is paired with.
resource "apple_app_info" "game" {
  app_id = data.apple_apps.example.apps[0].id

  primary_category        = "GAMES"
  primary_subcategory_one = "GAMES_PUZZLE"
  primary_subcategory_two = "GAMES_WORD"
  secondary_category      = "ENTERTAINMENT"
}
