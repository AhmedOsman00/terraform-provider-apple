# In-app purchases import as "<app_id>/<in_app_purchase_id>".
#
# The app ID is required because Apple never reports which app a purchase
# belongs to: InAppPurchaseV2 has no app relationship, and no include produces
# one. Importing with a bare ID would leave app_id null, and app_id forces
# replacement -- so the next plan would destroy the purchase and reserve its
# product ID forever.
terraform import apple_in_app_purchase.pro_unlock 6478123456/6739472901
