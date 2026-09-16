# Group localizations import by their own ID -- the provider reads them with
# include=subscriptionGroup, so the parent comes back with it. This is unlike
# apple_subscription_group itself, whose owning app Apple never reports.
terraform import apple_subscription_group_localization.premium_en 8812345601

# The composite form is accepted too, and additionally checks that the
# localization really belongs to the group named.
terraform import apple_subscription_group_localization.premium_en 21451234/8812345601
