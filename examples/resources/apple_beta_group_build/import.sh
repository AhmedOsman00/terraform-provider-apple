# An assignment is imported as "<group_id>/<pre_release_version>/<build_number>",
# naming the build the way the configuration does.
terraform import apple_beta_group_build.release_candidate "4f1a9c30-7b52-4e18-9d66-2c8a3f5e1b04/1.4.0/42"

# Add the platform second when the same train and build number exist on more
# than one platform.
terraform import apple_beta_group_build.release_candidate "4f1a9c30-7b52-4e18-9d66-2c8a3f5e1b04/IOS/1.4.0/42"

# A bare build ID is not accepted: it cannot be turned back into the build
# number and train without further requests.
