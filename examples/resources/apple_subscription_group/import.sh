# Subscription groups import as "<app_id>/<subscription_group_id>".
#
# The app ID is required because Apple never reports which app a group belongs
# to: GET /v1/subscriptionGroups/{id} accepts no "app" value for its include
# parameter. Find it with the apple_apps data source.
terraform import apple_subscription_group.premium 6448459855/21451234
