# A team member is imported by the address they sign in with.
terraform import apple_user.developer "ada@example.com"

# Apple's own identifier works too — the apple_users data source lists both.
terraform import apple_user.developer "1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"

# visible_apps imports as null for a member who can see every app: Apple would
# answer with the whole catalogue, which is not something a configuration set.
