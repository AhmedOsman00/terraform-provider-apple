data "apple_apps" "example" {
  bundle_id = "com.example.app"
}

# Apple computes the age rating from these answers; it is never set directly.
# The result appears as app_store_age_rating on apple_app_info.
#
# An unset answer is NOT the same as NONE -- Apple leaves an omitted attribute
# exactly as it was, which for a new app means unanswered, and an unanswered
# questionnaire blocks the submission. Answer every question that applies.
#
# This resource adopts the declaration Apple created with the app. It publishes
# neither POST nor DELETE for one, so terraform destroy drops it from state.
resource "apple_app_age_rating_declaration" "example" {
  app_id = data.apple_apps.example.apps[0].id

  # How often each kind of content appears. NONE, INFREQUENT_OR_MILD,
  # FREQUENT_OR_INTENSE, INFREQUENT or FREQUENT.
  alcohol_tobacco_or_drug_use_or_references        = "NONE"
  contests                                         = "NONE"
  gambling_simulated                               = "NONE"
  guns_or_other_weapons                            = "NONE"
  horror_or_fear_themes                            = "NONE"
  mature_or_suggestive_themes                      = "NONE"
  medical_or_treatment_information                 = "NONE"
  profanity_or_crude_humor                         = "NONE"
  sexual_content_graphic_and_nudity                = "NONE"
  sexual_content_or_nudity                         = "NONE"
  violence_cartoon_or_fantasy                      = "NONE"
  violence_realistic                               = "NONE"
  violence_realistic_prolonged_graphic_or_sadistic = "NONE"

  # Yes/no facts about what the app does. These belong to Apple's revised
  # questionnaire and did not exist on the original one.
  advertising                 = false
  age_assurance               = false
  gambling                    = false
  health_or_wellness_topics   = false
  loot_box                    = false
  messaging_and_chat          = false
  parental_controls           = false
  social_media                = false
  social_media_age_restricted = false
  unrestricted_web_access     = false
  user_generated_content      = false

  # An override can only raise the rating Apple computed, never lower it.
  age_rating_override_v2    = "NONE"
  korea_age_rating_override = "NONE"
}
