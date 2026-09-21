# The import ID is the app ID, not the detail ID: an app has exactly one beta
# review detail, and Apple publishes no collection of them.
#
# Apple also publishes no POST for the record — it comes into existence with the
# app — so importing and adopting on create end in the same place.
terraform import apple_beta_app_review_detail.example 6451234567
