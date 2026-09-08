data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_in_app_purchase" "pro_unlock" {
  app_id               = data.apple_apps.example.apps[0].id
  product_id           = "com.example.app.prounlock"
  name                 = "Pro Unlock"
  in_app_purchase_type = "NON_CONSUMABLE"
}

# A price is never a number: it references a price point from Apple's
# catalogue, which fixes the customer price and the developer proceeds for one
# territory. Always pass territories -- the unfiltered catalogue covers every
# storefront Apple sells in.
data "apple_in_app_purchase_price_points" "usa_9_99" {
  in_app_purchase_id = apple_in_app_purchase.pro_unlock.id
  territories        = ["USA"]
  customer_price     = "9.99"
}

data "apple_in_app_purchase_price_points" "egy" {
  in_app_purchase_id = apple_in_app_purchase.pro_unlock.id
  territories        = ["EGY"]
  customer_price     = "199"
}

# A purchase has exactly one price schedule. The base territory's price is
# equalized by Apple into every other storefront; each entry in prices then
# overrides the equalized price for the territory its price point belongs to.
#
# This resource cannot be destroyed: Apple publishes no DELETE for a price
# schedule, so terraform destroy drops it from state and warns.
resource "apple_in_app_purchase_price_schedule" "pro_unlock" {
  in_app_purchase_id = apple_in_app_purchase.pro_unlock.id
  base_territory     = "USA"

  prices = [
    {
      price_point_id = data.apple_in_app_purchase_price_points.usa_9_99.price_points[0].id
    },
    # A local price that overrides what Apple would have equalized, taking
    # effect on a date. Dates are plain dates, never timestamps.
    {
      price_point_id = data.apple_in_app_purchase_price_points.egy.price_points[0].id
      start_date     = "2026-10-01"
    },
  ]
}
