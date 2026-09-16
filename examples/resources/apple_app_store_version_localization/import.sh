# A bare Apple localization ID -- the client asks for include=appStoreVersion,
# so the parent comes back with it.
terraform import apple_app_store_version_localization.en 3f9a7c25-6b81-4e0d-93af-5c2d8e1b74a0

# Or the composite form.
terraform import apple_app_store_version_localization.en 8a41f2c7-93b6-4e5d-a1c8-2f7e6b4d0931/en-US
