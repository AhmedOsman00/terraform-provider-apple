data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

data "apple_subscription_groups" "premium" {
  app_id = data.apple_apps.example.apps[0].id
}

data "apple_subscriptions" "pro_monthly" {
  group_id           = data.apple_subscription_groups.premium.subscription_groups[0].id
  product_id_pattern = "^com\\.example\\.app\\.pro\\.monthly$"
}

# Step one: pick the one price you actually reason about. Everything else is
# derived from it, so this is the only number in the configuration.
data "apple_subscription_price_points" "base" {
  subscription_id = data.apple_subscriptions.pro_monthly.subscriptions[0].id
  territories     = ["USA"]
  customer_price  = "9.99"
}

# Step two: ask Apple what that price is worth everywhere else. This is the
# mapping App Store Connect's own price matrix is built from -- there is no
# customer_price to filter on per territory, because 9.99 in the United States
# is neither 9.99 nor a round number anywhere else.
data "apple_subscription_price_point_equalizations" "base" {
  price_point_id = data.apple_subscription_price_points.base.price_points[0].id
}

# Narrowed to the storefronts a plan actually sells in. The filter is passed to
# Apple; unlike the catalogue read, leaving it off is the ordinary case here,
# because this endpoint returns one record per territory rather than tens of
# thousands.
data "apple_subscription_price_point_equalizations" "europe" {
  price_point_id = data.apple_subscription_price_points.base.price_points[0].id
  territories    = ["GBR", "DEU", "FRA", "ITA", "ESP"]
}

# price_point_ids is a territory-code-to-ID map, ready to drive a for_each over
# apple_subscription_price. Reaching the same IDs through price_points needs a
# flatten and a one-element index expression in every configuration.
output "price_point_ids" {
  value = data.apple_subscription_price_point_equalizations.base.price_point_ids
}

# What the equalization actually charges, for review before it is applied.
output "prices" {
  value = {
    for point in data.apple_subscription_price_point_equalizations.europe.price_points :
    point.territory_id => point.customer_price
  }
}
