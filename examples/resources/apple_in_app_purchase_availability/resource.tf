data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_in_app_purchase" "pro_unlock" {
  app_id               = data.apple_apps.example.apps[0].id
  product_id           = "com.example.app.prounlock"
  name                 = "Pro Unlock"
  in_app_purchase_type = "NON_CONSUMABLE"
}

# A purchase has exactly one availability record, and stays in MISSING_METADATA
# until it has one. Territories are named by three-letter Apple code -- USA,
# GBR, EGY -- not by the two-letter ISO 3166-1 alpha-2 code.
#
# This resource cannot be destroyed: Apple publishes neither PATCH nor DELETE
# for an availability, so an update is a POST that supersedes the previous
# record, and terraform destroy drops it from state and warns. To stop selling
# in a territory, remove it from available_territories.
resource "apple_in_app_purchase_availability" "pro_unlock" {
  in_app_purchase_id           = apple_in_app_purchase.pro_unlock.id
  available_in_new_territories = true

  available_territories = [
    "USA",
    "GBR",
    "DEU",
    "FRA",
    "EGY",
  ]
}
