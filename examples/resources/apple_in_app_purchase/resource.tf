data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# A one-time purchase, bought once and owned forever. This is the shape a
# "pro unlock" or "remove ads" takes -- for a recurring charge, use
# apple_subscription instead.
#
# A purchase on its own is not sellable: Apple reports it as MISSING_METADATA
# until it has a localization, a price schedule and an availability. All three
# are separate resources.
resource "apple_in_app_purchase" "pro_unlock" {
  app_id               = data.apple_apps.example.apps[0].id
  product_id           = "com.example.app.prounlock"
  name                 = "Pro Unlock"
  in_app_purchase_type = "NON_CONSUMABLE"
  family_sharable      = true
  review_note          = "Sign in with the demo account, then open Settings > Unlock Pro."
}

# A consumable is bought over and over -- credits, coins, a refill. Family
# Sharing does not apply to consumables, so leave it off.
resource "apple_in_app_purchase" "coins_100" {
  app_id               = data.apple_apps.example.apps[0].id
  product_id           = "com.example.app.coins100"
  name                 = "100 Coins"
  in_app_purchase_type = "CONSUMABLE"
}

# A non-renewing subscription is a fixed-term entitlement the customer has to
# buy again by hand when it lapses. Apple does not renew it and StoreKit does
# not track its expiry -- the app does.
resource "apple_in_app_purchase" "season_pass" {
  app_id               = data.apple_apps.example.apps[0].id
  product_id           = "com.example.app.season"
  name                 = "Season Pass"
  in_app_purchase_type = "NON_RENEWING_SUBSCRIPTION"
}
