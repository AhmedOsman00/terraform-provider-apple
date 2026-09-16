# A bare Apple version ID.
terraform import apple_app_store_version.v1 8a41f2c7-93b6-4e5d-a1c8-2f7e6b4d0931

# Or the composite form -- all three parts, because the same version string
# exists once per platform.
terraform import apple_app_store_version.v1 6478291043/IOS/1.0
