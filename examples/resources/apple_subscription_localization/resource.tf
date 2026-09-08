data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "premium" {
  app_id         = data.apple_apps.example.apps[0].id
  reference_name = "Premium"
}

resource "apple_subscription" "pro_monthly" {
  group_id            = apple_subscription_group.premium.id
  product_id          = "com.example.app.pro.monthly"
  name                = "Pro Monthly"
  subscription_period = "ONE_MONTH"
}

# This is what a customer actually reads on the App Store and in the system
# purchase sheet. The name on apple_subscription is an internal reference name
# that customers never see.
resource "apple_subscription_localization" "pro_monthly_en" {
  subscription_id = apple_subscription.pro_monthly.id
  locale          = "en-US"
  name            = "Pro Monthly"
  description     = "Everything in Pro, billed monthly."
}

# One localization per locale. The locale must be one the app itself supports:
# Apple rejects a localization for a locale the app has not been localized into.
resource "apple_subscription_localization" "pro_monthly_ar" {
  subscription_id = apple_subscription.pro_monthly.id
  locale          = "ar-SA"
  name            = "برو شهري"
  description     = "كل مزايا برو، باشتراك شهري."
}
