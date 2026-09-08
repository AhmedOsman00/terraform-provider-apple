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

# Always pass territories. The unfiltered catalogue covers every territory the
# App Store sells in and runs to tens of thousands of records; the filter is
# passed to Apple rather than applied in memory.
data "apple_subscription_price_points" "usa" {
  subscription_id = data.apple_subscriptions.pro_monthly.subscriptions[0].id
  territories     = ["USA"]
}

# Narrow to one price. customer_price is matched exactly, after Apple's
# territory filter -- note that the same number means a different amount of
# money in each territory's currency.
data "apple_subscription_price_points" "usa_9_99" {
  subscription_id = data.apple_subscriptions.pro_monthly.subscriptions[0].id
  territories     = ["USA"]
  customer_price  = "9.99"
}

# Several territories at once, for a price schedule that covers a region.
data "apple_subscription_price_points" "region" {
  subscription_id = data.apple_subscriptions.pro_monthly.subscriptions[0].id
  territories     = ["USA", "GBR", "EGY", "DEU"]
  limit           = 100
}
