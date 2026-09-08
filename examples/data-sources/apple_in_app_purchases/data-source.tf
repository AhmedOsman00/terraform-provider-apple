data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# app_id is required: Apple publishes no top-level in-app purchase collection,
# so a purchase is reachable only through the app that owns it.
data "apple_in_app_purchases" "all" {
  app_id = data.apple_apps.example.apps[0].id
}

# Filter by kind. Auto-renewable subscriptions are not in here at all -- they
# are a different Apple resource, listed by apple_subscriptions.
data "apple_in_app_purchases" "consumables" {
  app_id               = data.apple_apps.example.apps[0].id
  in_app_purchase_type = "CONSUMABLE"
  sort_by              = "product_id"
}

# Patterns are regular expressions, matched as a substring unless anchored.
data "apple_in_app_purchases" "pro_unlock" {
  app_id             = data.apple_apps.example.apps[0].id
  product_id_pattern = "^com\\.example\\.app\\.prounlock$"
}

# Purchases Apple is still waiting on metadata for.
data "apple_in_app_purchases" "incomplete" {
  app_id = data.apple_apps.example.apps[0].id
  state  = "MISSING_METADATA"
}

output "in_app_purchase_count" {
  value = data.apple_in_app_purchases.all.total_count
}
