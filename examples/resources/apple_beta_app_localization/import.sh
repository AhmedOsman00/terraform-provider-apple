# A bare Apple localization ID: the client asks for include=app, so the owning
# app comes back with the record.
terraform import 'apple_beta_app_localization.page["en-US"]' 9c2f7a41-30b8-4d65-8e19-5a7b2c4f6d38

# Or by app and locale, which is what a person actually has.
terraform import 'apple_beta_app_localization.page["en-US"]' 6451234567/en-US
