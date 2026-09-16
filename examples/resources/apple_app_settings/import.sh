# The import ID is the app's Apple ID, which the apple_apps data source reports.
terraform import apple_app_settings.example 6478291043

# A bundle identifier is accepted too, and is distinguished by its shape --
# Apple's own app IDs are all digits.
terraform import apple_app_settings.example com.example.app
