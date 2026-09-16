data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

resource "apple_app_store_version" "v1" {
  app_id         = data.apple_apps.example.apps[0].id
  platform       = "IOS"
  version_string = "1.0"
  copyright      = "2026 AO Studio"
}

# The per-release half of the localized metadata. The app's name and subtitle
# outlive the release and belong to apple_app_info_localization.
#
# keywords is a list here and a single comma-separated string at Apple, capped
# at 100 characters including the commas -- the provider joins the list with no
# spaces and checks the joined length at plan time.
resource "apple_app_store_version_localization" "en" {
  app_store_version_id = apple_app_store_version.v1.id
  locale               = "en-US"

  description = <<-EOT
    Fenn keeps your money on your phone.

    Every record lives in an encrypted database on this iPhone. No account is
    required, and signing in later only moves your ledger to a second device.
  EOT

  keywords = [
    "budget",
    "expenses",
    "spending",
    "receipts",
    "money",
    "offline",
    "private",
  ]

  promotional_text = "Your spending stays on your iPhone, encrypted."
  marketing_url    = "https://example.com"
  support_url      = "https://example.com/support"

  # Apple rejects whats_new on an app's first version -- there is nothing new
  # about it -- and requires it on every version after.
  # whats_new = "Faster receipt scanning and a new monthly dashboard."
}

resource "apple_app_store_version_localization" "ar" {
  app_store_version_id = apple_app_store_version.v1.id
  locale               = "ar-SA"

  description      = "‏Fenn يحفظ أموالك على جهازك."
  promotional_text = "مصروفاتك تبقى على جهازك مشفّرة."
  support_url      = "https://example.com/ar/support"

  keywords = [
    "ميزانية",
    "مصروفات",
    "نفقات",
    "إيصالات",
  ]
}
