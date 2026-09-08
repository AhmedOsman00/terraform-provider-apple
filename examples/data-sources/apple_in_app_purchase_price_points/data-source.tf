data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

data "apple_in_app_purchases" "pro_unlock" {
  app_id             = data.apple_apps.example.apps[0].id
  product_id_pattern = "^com\\.example\\.app\\.prounlock$"
}

# Always pass territories. The unfiltered catalogue covers every territory the
# App Store sells in and runs to tens of thousands of records; the filter is
# passed to Apple rather than applied in memory.
data "apple_in_app_purchase_price_points" "usa" {
  in_app_purchase_id = data.apple_in_app_purchases.pro_unlock.in_app_purchases[0].id
  territories        = ["USA"]
}

# Narrow to one price. customer_price is matched exactly, after Apple's
# territory filter -- note that the same number means a different amount of
# money in each territory's currency.
data "apple_in_app_purchase_price_points" "usa_9_99" {
  in_app_purchase_id = data.apple_in_app_purchases.pro_unlock.in_app_purchases[0].id
  territories        = ["USA"]
  customer_price     = "9.99"
}

# Several territories at once, for a schedule that overrides Apple's equalized
# prices across a region.
data "apple_in_app_purchase_price_points" "region" {
  in_app_purchase_id = data.apple_in_app_purchases.pro_unlock.in_app_purchases[0].id
  territories        = ["USA", "GBR", "EGY", "DEU"]
  limit              = 100
}
