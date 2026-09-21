# A bare Apple group ID. Unlike apple_subscription_group, this works: GET
# /v1/betaGroups/{id} accepts include=app, so the owning app comes back with the
# record and app_id can be written from it.
terraform import apple_beta_group.qa 4f1a9c30-7b52-4e18-9d66-2c8a3f5e1b04

# Or by name, which is what a person actually has. Names are unique within an
# app.
terraform import apple_beta_group.qa 6451234567/QA

# Note that is_internal_group is absent from Apple's update request, so a group
# imported as internal plans a replacement if the configuration does not say
# is_internal_group = true.
