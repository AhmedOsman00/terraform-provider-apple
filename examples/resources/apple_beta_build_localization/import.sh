# The import ID is composite and there is no bare-ID form: the record names its
# build by Apple's opaque ID, from which the build number and its train cannot be
# recovered without further requests — while a person importing already has both.
terraform import 'apple_beta_build_localization.whats_new["en-US"]' 6451234567/1.4.0/42/en-US

# Add the platform as the second part when the same train and build number exist
# on more than one platform.
terraform import 'apple_beta_build_localization.whats_new["en-US"]' 6451234567/IOS/1.4.0/42/en-US
