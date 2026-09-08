# Subscription prices import as "<subscription_id>/<price_id>".
#
# The subscription ID is required because Apple publishes no GET for a single
# subscription price: a price is only reachable through the collection hanging
# off its subscription.
terraform import apple_subscription_price.pro_monthly_usa 6739472901/9923456701
