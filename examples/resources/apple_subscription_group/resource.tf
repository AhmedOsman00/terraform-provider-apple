# A subscription group holds the subscriptions a customer chooses between. Only
# one subscription per group can be active at a time, which is what makes
# upgrades and downgrades possible.
#
# The group belongs to an app record. Apple's API cannot create one -- its
# documentation says to create new apps on the App Store Connect website -- so
# the app is looked up rather than managed.
data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_subscription_group" "premium" {
  app_id         = data.apple_apps.example.apps[0].id
  reference_name = "Premium"
}
