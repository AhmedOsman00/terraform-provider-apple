# The composite form is what you are likely to have to hand.
terraform import apple_app_info_localization.en 6478291043/en-US

# A bare Apple localization ID works too: the provider resolves the app in two
# hops, localization to app info, app info to app.
terraform import apple_app_info_localization.en 4c2bd18e-5f3a-4d21-9d8e-1b7f0a3c9e64
