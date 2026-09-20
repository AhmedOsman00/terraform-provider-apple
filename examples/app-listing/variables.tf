variable "bundle_id" {
  description = <<-EOT
    The bundle identifier of an app that already exists in App Store Connect.

    Apple's API cannot create an app record -- its documentation says to create
    apps on the App Store Connect website -- so this module looks one up.
  EOT
  type        = string
}

variable "platform" {
  description = "The platform this release targets: IOS, MAC_OS, TV_OS or VISION_OS."
  type        = string
  default     = "IOS"

  validation {
    condition     = contains(["IOS", "MAC_OS", "TV_OS", "VISION_OS"], var.platform)
    error_message = "platform must be one of IOS, MAC_OS, TV_OS or VISION_OS."
  }
}

variable "version_string" {
  description = "The version number customers see, such as 1.0 or 2.14.3."
  type        = string
}

variable "build_number" {
  description = <<-EOT
    The build number of the uploaded build to attach -- CFBundleVersion, which
    a pipeline knows as CURRENT_PROJECT_VERSION.

    This module does not upload builds; the binary must already be in App Store
    Connect, put there by Xcode, Transporter or fastlane. Apple rejects a build
    that is still processing, which takes five to thirty minutes after the
    upload finishes, so a pipeline that uploads and applies in one run should
    wait in between.

    Left null, whatever build is attached is left alone -- including one
    attached by hand or by a separate pipeline.
  EOT
  type        = string
  default     = null

  validation {
    condition     = var.build_number == null || can(regex("^\\d+(\\.\\d+)*$", var.build_number))
    error_message = "build_number must be dot-separated numbers, such as 42 or 1.2.3."
  }
}

variable "copyright" {
  description = <<-EOT
    The copyright line shown on the product page -- "2026 AO Studio".

    Apple wants the year and the rights holder, not a copyright symbol, which
    it adds itself.
  EOT
  type        = string
  default     = null
}

variable "automatic_release" {
  description = <<-EOT
    Whether an approved version is released the moment it passes review.

    false holds it until it is released by hand in App Store Connect. This is
    the release_type attribute in disguise: true is AFTER_APPROVAL, false is
    MANUAL. A scheduled release needs release_type = "SCHEDULED" and an
    earliest_release_date, which this module does not expose.
  EOT
  type        = bool
  default     = false
}

variable "primary_locale" {
  description = <<-EOT
    The app's primary language, as an App Store locale code such as en-US.

    This is the language App Review falls back to, and the one whose
    localization cannot be deleted.
  EOT
  type        = string
  default     = "en-US"
}

variable "content_rights_declaration" {
  description = <<-EOT
    Whether the app contains, shows or accesses third-party content.

    This is App Store Connect's Content Rights Information question, which
    blocks a submission until it is answered.
  EOT
  type        = string
  default     = "DOES_NOT_USE_THIRD_PARTY_CONTENT"

  validation {
    condition = contains(
      ["DOES_NOT_USE_THIRD_PARTY_CONTENT", "USES_THIRD_PARTY_CONTENT"],
      var.content_rights_declaration
    )
    error_message = "content_rights_declaration must be DOES_NOT_USE_THIRD_PARTY_CONTENT or USES_THIRD_PARTY_CONTENT."
  }
}

variable "primary_category" {
  description = <<-EOT
    The app's primary App Store category, as Apple's own constant -- FINANCE,
    PRODUCTIVITY, GAMES_PUZZLE.

    List the valid values with the apple_app_categories data source.
  EOT
  type        = string
}

variable "secondary_category" {
  description = "The app's secondary category, or null to list in one category only."
  type        = string
  default     = null
}

variable "locales" {
  description = <<-EOT
    The localized metadata, keyed by App Store locale code.

    Apple splits these across two records with two lifetimes, and this module
    fans one map out to both: title, subtitle and privacy_policy_url describe
    the app and survive every release, while description, keywords, promo_text
    and whats_new describe one release and are replaced with it.

    Lengths Apple enforces, all counted in characters rather than bytes:
    title and subtitle 30, promo_text 170, description and whats_new 4000.
    keywords is capped at 100 characters once joined with commas, which the
    provider checks at plan time.
  EOT

  type = map(object({
    title              = string
    subtitle           = optional(string)
    privacy_policy_url = optional(string)
    description        = optional(string)
    keywords           = optional(list(string))
    promo_text         = optional(string)
    marketing_url      = optional(string)
    support_url        = optional(string)
    whats_new          = optional(string)
  }))

  validation {
    condition     = length(var.locales) > 0
    error_message = "At least one locale is required -- the app's primary language."
  }
}

variable "review" {
  description = <<-EOT
    What App Review is told: who to contact, how to sign in, and anything a
    reviewer needs to know.

    demo_account_password is stored in Terraform state in plain text. Supply it
    from a secret store, and use an account created for App Review alone.
  EOT

  type = object({
    first_name            = optional(string)
    last_name             = optional(string)
    email                 = optional(string)
    phone                 = optional(string)
    demo_required         = optional(bool, false)
    demo_account_name     = optional(string)
    demo_account_password = optional(string)
    notes                 = optional(string)
  })

  sensitive = true
  default   = {}

  validation {
    condition = (
      !coalesce(var.review.demo_required, false) ||
      (var.review.demo_account_name != null && var.review.demo_account_password != null)
    )
    error_message = "When review.demo_required is true, both demo_account_name and demo_account_password are required -- Apple rejects the review detail otherwise."
  }
}

variable "advisory" {
  description = <<-EOT
    The answered content questionnaire behind the app's age rating.

    An unset answer is not NONE: Apple leaves an omitted attribute exactly as it
    was, which for a new app means unanswered, and an unanswered questionnaire
    blocks the submission. The defaults here answer NONE and false to
    everything, which is right for an app with no mature content and wrong to
    leave unread.

    The frequency answers take NONE, INFREQUENT_OR_MILD, FREQUENT_OR_INTENSE,
    INFREQUENT or FREQUENT. The first two of those belong to Apple's original
    questionnaire and the last two to the revised one.
  EOT

  type = object({
    alcohol_tobacco_or_drug_use_or_references        = optional(string, "NONE")
    contests                                         = optional(string, "NONE")
    gambling_simulated                               = optional(string, "NONE")
    guns_or_other_weapons                            = optional(string, "NONE")
    horror_or_fear_themes                            = optional(string, "NONE")
    mature_or_suggestive_themes                      = optional(string, "NONE")
    medical_or_treatment_information                 = optional(string, "NONE")
    profanity_or_crude_humor                         = optional(string, "NONE")
    sexual_content_graphic_and_nudity                = optional(string, "NONE")
    sexual_content_or_nudity                         = optional(string, "NONE")
    violence_cartoon_or_fantasy                      = optional(string, "NONE")
    violence_realistic                               = optional(string, "NONE")
    violence_realistic_prolonged_graphic_or_sadistic = optional(string, "NONE")

    advertising                 = optional(bool, false)
    age_assurance               = optional(bool, false)
    gambling                    = optional(bool, false)
    health_or_wellness_topics   = optional(bool, false)
    loot_box                    = optional(bool, false)
    messaging_and_chat          = optional(bool, false)
    parental_controls           = optional(bool, false)
    social_media                = optional(bool, false)
    social_media_age_restricted = optional(bool, false)
    unrestricted_web_access     = optional(bool, false)
    user_generated_content      = optional(bool, false)

    age_rating_override_v2    = optional(string, "NONE")
    korea_age_rating_override = optional(string, "NONE")
  })

  default = {}
}

variable "base_territory" {
  description = <<-EOT
    The three-letter Apple territory code whose price Apple equalizes into every
    other storefront -- USA, GBR, EGY.

    Not the two-letter ISO 3166-1 alpha-2 code.
  EOT
  type        = string
  default     = "USA"
}

variable "customer_price" {
  description = <<-EOT
    What the app costs in the base territory, as a plain decimal string without
    a currency symbol -- "9.99", or "0" for a free app.

    Matched against Apple's price point catalogue, which is why it is a string
    rather than a number: "9.99" and "9.990" are not the same record.
  EOT
  type        = string
  default     = "0"
}

variable "territories" {
  description = <<-EOT
    The storefronts to sell in, as three-letter Apple territory codes.

    Leave it empty to sell in every storefront the App Store operates in, read
    from the apple_territories data source.
  EOT
  type        = list(string)
  default     = []
}
