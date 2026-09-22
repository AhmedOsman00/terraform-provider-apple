# A pending invitation is imported by the address it was sent to.
terraform import apple_user_invitation.developer "ada@example.com"

# Apple's own identifier works too.
terraform import apple_user_invitation.developer "5f2b8d41-6c93-4a27-8e15-3d9b7a2c4e60"

# Only a pending invitation can be imported. An accepted one no longer exists at
# Apple — import the member it created as an apple_user instead.
