# The import ID is the subscription ID. A subscription has no price schedule
# record of its own at Apple -- unlike an in-app purchase -- so its prices are
# reachable only through the collection hanging off it.
terraform import apple_subscription_price_schedule.pro_annual 6740021496
