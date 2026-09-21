data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# The half of TestFlight metadata that survives every build: what the app is,
# where feedback goes, and the links shown beside it. The per-build "What to
# Test" note is apple_beta_build_localization.
locals {
  testflight = {
    "en-US" = {
      description = <<-EOT
        Track what you spend by typing, by voice, or by photographing a receipt.
        This beta adds shared budgets and a rewritten sync engine — please try
        both on more than one device.
      EOT
    }
    "ar-SA" = {
      description = <<-EOT
        تتبّع مصروفاتك بالكتابة أو بالصوت أو بتصوير الإيصال. تضيف هذه النسخة
        التجريبية الميزانيات المشتركة ومحرك مزامنة جديد — جرّب كليهما على أكثر
        من جهاز.
      EOT
    }
  }
}

resource "apple_beta_app_localization" "page" {
  for_each = local.testflight

  app_id      = data.apple_apps.example.apps[0].id
  locale      = each.key
  description = each.value.description

  # Prefer a shared inbox: external testers see this address.
  feedback_email = "testflight@example.com"
  marketing_url  = "https://example.com"

  # The TestFlight privacy policy link, which is not the App Store one on
  # apple_app_info_localization and not the App Privacy questionnaire — that
  # has no API at all.
  privacy_policy_url = "https://example.com/privacy"
}
