# Membership is imported as "<group_id>/<email>".
terraform import 'apple_beta_tester.early_access["ada@example.com"]' "4f1a9c30-7b52-4e18-9d66-2c8a3f5e1b04/ada@example.com"

# A bare tester ID is not accepted. Apple's ID names the person, not the
# membership, and the same ID belongs to every group that person is in — so it
# cannot say which membership to import.
#
# first_name and last_name import as null. Apple reports both, but it accepts
# neither after the record exists, so adopting a name would plan a replacement
# that could not change it.
