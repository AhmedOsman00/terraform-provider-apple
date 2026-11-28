data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

data "apple_in_app_purchases" "coins" {
  app_id             = data.apple_apps.example.apps[0].id
  product_id_pattern = "^com\\.example\\.app\\.coins$"
}

# The one price you reason about.
data "apple_in_app_purchase_price_points" "base" {
  in_app_purchase_id = data.apple_in_app_purchases.coins.in_app_purchases[0].id
  territories        = ["USA"]
  customer_price     = "4.99"
}

# What Apple's equalization of that price charges everywhere else. A price
# schedule with base_territory = "USA" already sells at these prices, so this
# is for reviewing them, or for pinning them as manual prices.
data "apple_in_app_purchase_price_point_equalizations" "base" {
  price_point_id = data.apple_in_app_purchase_price_points.base.price_points[0].id
}

# Narrowed to a region. The filter is passed to Apple.
data "apple_in_app_purchase_price_point_equalizations" "europe" {
  price_point_id = data.apple_in_app_purchase_price_points.base.price_points[0].id
  territories    = ["GBR", "DEU", "FRA", "ITA", "ESP"]
}

output "europe_prices" {
  value = {
    for point in data.apple_in_app_purchase_price_point_equalizations.europe.price_points :
    point.territory_id => point.customer_price
  }
}

# Pin every territory at today's equalization. Apple moves an equalized price
# when exchange rates or taxes change; a manual price stays where it was set.
resource "apple_in_app_purchase_price_schedule" "coins" {
  in_app_purchase_id = data.apple_in_app_purchases.coins.in_app_purchases[0].id
  base_territory     = "USA"

  # Whether Apple includes the base territory in the equalizations is Apple's
  # call, so merge it in rather than assume.
  prices = [
    for point in values(merge(
      data.apple_in_app_purchase_price_point_equalizations.base.price_point_ids,
      { USA = data.apple_in_app_purchase_price_points.base.price_points[0].id },
    )) : { price_point_id = point }
  ]
}
