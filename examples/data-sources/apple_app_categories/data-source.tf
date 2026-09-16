# Apple's category catalogue is fixed and the same for every account. A
# category's ID is the constant itself -- FINANCE, PRODUCTIVITY, GAMES_PUZZLE --
# which is what apple_app_info takes, so this exists to discover the valid
# values rather than to look one up at apply time.
data "apple_app_categories" "all" {}

# Only a top-level category can be an app's primary or secondary category.
data "apple_app_categories" "assignable" {
  platforms      = ["IOS"]
  top_level_only = true
}

# id_pattern is a regular expression, matched as a substring unless anchored.
data "apple_app_categories" "games" {
  id_pattern = "^GAMES"
}

output "category_ids" {
  value = [for category in data.apple_app_categories.assignable.categories : category.id]
}

# A category's subcategories, for the handful that have any.
output "games_subcategories" {
  value = {
    for category in data.apple_app_categories.games.categories :
    category.id => category.subcategories if length(category.subcategories) > 0
  }
}
