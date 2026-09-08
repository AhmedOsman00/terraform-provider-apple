# Look up one app by its bundle identifier. This is the usual form: a
# subscription group needs the app's Apple ID, and the bundle identifier is what
# you already know.
data "apple_apps" "fenn" {
  bundle_id = "com.example.app"
}

locals {
  app_id = data.apple_apps.fenn.apps[0].id
}

# List every app on the account.
data "apple_apps" "all" {}

# Filter by name. The pattern is a regular expression matched as a substring,
# so anchor it with ^ and $ for an exact match.
data "apple_apps" "beta_builds" {
  name_pattern = "(?i)beta$"
}

# Filter by SKU, sort, and cap the result.
data "apple_apps" "sorted" {
  sku        = "APP-1"
  sort_by    = "bundle_id"
  sort_order = "asc"
  limit      = 10
}
