data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

data "apple_subscription_groups" "premium" {
  app_id                 = data.apple_apps.example.apps[0].id
  reference_name_pattern = "^Premium$"
}

# group_id is required: a subscription is reachable only through its group.
data "apple_subscriptions" "all" {
  group_id = data.apple_subscription_groups.premium.subscription_groups[0].id
}

# state and subscription_period are exact matches on Apple's enums. The two
# pattern filters are regular expressions matched as substrings.
data "apple_subscriptions" "live_yearly" {
  group_id            = data.apple_subscription_groups.premium.subscription_groups[0].id
  state               = "APPROVED"
  subscription_period = "ONE_YEAR"
}

data "apple_subscriptions" "pro_tier" {
  group_id           = data.apple_subscription_groups.premium.subscription_groups[0].id
  product_id_pattern = "\\.pro\\."
  sort_by            = "group_level"
  sort_order         = "asc"
  limit              = 10
}

# Anything still missing a localization or a price shows up here.
data "apple_subscriptions" "incomplete" {
  group_id = data.apple_subscription_groups.premium.subscription_groups[0].id
  state    = "MISSING_METADATA"
}
