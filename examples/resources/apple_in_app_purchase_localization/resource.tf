data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_in_app_purchase" "pro_unlock" {
  app_id               = data.apple_apps.example.apps[0].id
  product_id           = "com.example.app.prounlock"
  name                 = "Pro Unlock"
  in_app_purchase_type = "NON_CONSUMABLE"
}

# This is what a customer actually reads on the App Store and in the system
# purchase sheet. The name on apple_in_app_purchase is an internal reference
# name that customers never see.
#
# Apple attaches localizations to an in-app purchase version rather than to the
# purchase itself; the provider resolves the version and reports it as
# version_id.
resource "apple_in_app_purchase_localization" "pro_unlock_en" {
  in_app_purchase_id = apple_in_app_purchase.pro_unlock.id
  locale             = "en-US"
  name               = "Pro Unlock"
  description        = "Unlock every Pro feature, once and for all."
}

# One localization per locale. The locale must be one the app itself supports:
# Apple rejects a localization for a locale the app has not been localized into.
resource "apple_in_app_purchase_localization" "pro_unlock_ar" {
  in_app_purchase_id = apple_in_app_purchase.pro_unlock.id
  locale             = "ar-SA"
  name               = "فتح برو"
  description        = "افتح كل مزايا برو مرة واحدة وللأبد."
}
