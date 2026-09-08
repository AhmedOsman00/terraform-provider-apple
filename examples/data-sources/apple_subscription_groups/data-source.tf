data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# app_id is required: Apple publishes no top-level subscription group
# collection, so a group is reachable only through the app that owns it.
data "apple_subscription_groups" "all" {
  app_id = data.apple_apps.example.apps[0].id
}

# reference_name_pattern is a regular expression matched as a substring.
data "apple_subscription_groups" "premium" {
  app_id                 = data.apple_apps.example.apps[0].id
  reference_name_pattern = "^Premium$"
}

data "apple_subscription_groups" "sorted" {
  app_id     = data.apple_apps.example.apps[0].id
  sort_by    = "reference_name"
  sort_order = "desc"
  limit      = 5
}
