# Localizations import by their own ID -- the provider reads them with
# include=subscription, so the parent comes back with it.
terraform import apple_subscription_localization.pro_monthly_en 8812345601

# The composite form is accepted too, and additionally checks that the
# localization really belongs to the subscription named.
terraform import apple_subscription_localization.pro_monthly_en 6739472901/8812345601
