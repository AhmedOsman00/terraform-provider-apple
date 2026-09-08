# Localizations import by their own ID. Apple reports the version that owns a
# localization, and the version reports the purchase, so the provider resolves
# the parent in two hops.
terraform import apple_in_app_purchase_localization.pro_unlock_en 8812345601

# The composite form is accepted too, and additionally checks that the
# localization really belongs to the in-app purchase named.
terraform import apple_in_app_purchase_localization.pro_unlock_en 6739472901/8812345601
