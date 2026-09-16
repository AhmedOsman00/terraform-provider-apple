data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# Always set territories. The unfiltered catalogue covers every storefront the
# App Store sells in and runs to tens of thousands of records; the filter is
# passed to Apple rather than applied in memory, so it is the difference
# between one request and hundreds.
data "apple_app_price_points" "usa" {
  app_id         = data.apple_apps.example.apps[0].id
  territories    = ["USA"]
  customer_price = "9.99"
}

# The zero price point is what makes an app free.
data "apple_app_price_points" "free" {
  app_id         = data.apple_apps.example.apps[0].id
  territories    = ["USA"]
  customer_price = "0"
}

# Price point IDs are scoped to the app they were read from, and neither a
# subscription's nor an in-app purchase's price points work here.
output "usa_price_point" {
  value = data.apple_app_price_points.usa.price_points[0].id
}

output "usa_proceeds" {
  value = data.apple_app_price_points.usa.price_points[0].proceeds
}
