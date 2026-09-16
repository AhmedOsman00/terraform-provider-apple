data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# A price is never written as a number: it references one of Apple's price
# points, each fixing a customer price and the developer proceeds for one
# territory. Always scope the lookup to the territories you need -- the
# unfiltered catalogue covers every storefront Apple sells in.
data "apple_app_price_points" "usa" {
  app_id         = data.apple_apps.example.apps[0].id
  territories    = ["USA"]
  customer_price = "9.99"
}

# This is App Store Connect's Pricing section -- the one that blocks a
# submission with "You must choose a price tier in Pricing". Apple retired
# price tiers: the price is a base territory Apple equalizes into every other
# storefront, plus any manual overrides.
#
# This resource cannot be destroyed: Apple publishes no DELETE for a price
# schedule, so an app that has been priced stays priced.
resource "apple_app_price_schedule" "example" {
  app_id         = data.apple_apps.example.apps[0].id
  base_territory = "USA"

  prices = [
    {
      price_point_id = data.apple_app_price_points.usa.price_points[0].id
    },
  ]
}

# A free app is a price point of 0, not the absence of a schedule.
data "apple_app_price_points" "free" {
  app_id         = data.apple_apps.example.apps[0].id
  territories    = ["USA"]
  customer_price = "0"
}

resource "apple_app_price_schedule" "free" {
  app_id         = data.apple_apps.example.apps[0].id
  base_territory = "USA"

  prices = [
    {
      price_point_id = data.apple_app_price_points.free.price_points[0].id
    },
  ]
}
