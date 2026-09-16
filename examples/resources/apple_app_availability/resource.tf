data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# available_in_new_territories only covers storefronts Apple opens later, so it
# is no substitute for naming the current ones.
resource "apple_app_availability" "example" {
  app_id                       = data.apple_apps.example.apps[0].id
  available_in_new_territories = true

  territories = [
    { territory = "USA" },
    { territory = "GBR" },
    { territory = "DEU" },
    { territory = "EGY" },
  ]
}

# To sell everywhere, read the storefront list rather than typing it out.
data "apple_territories" "all" {}

resource "apple_app_availability" "worldwide" {
  app_id                       = data.apple_apps.example.apps[0].id
  available_in_new_territories = true

  territories = [
    for code in data.apple_territories.all.ids : { territory = code }
  ]
}

# A staggered launch, with pre-orders open in the storefront that ships later.
resource "apple_app_availability" "staggered" {
  app_id = data.apple_apps.example.apps[0].id

  territories = [
    { territory = "USA" },
    {
      territory         = "GBR"
      release_date      = "2026-04-15"
      pre_order_enabled = true
    },
  ]
}
