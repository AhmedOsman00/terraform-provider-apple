# The import ID is the in-app purchase ID, not the availability ID: a purchase
# has exactly one availability record, and Apple publishes no collection of them
# to look one up in.
terraform import apple_in_app_purchase_availability.pro_unlock 6739472901
