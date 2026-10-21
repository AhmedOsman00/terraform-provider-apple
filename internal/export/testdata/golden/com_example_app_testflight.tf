# TestFlight for Example (app ID 6478123456).
# Removing an apple_beta_tester takes that person out of the group.

import {
  to = apple_beta_group.com_example_app_team
  id = "BG1"
}
resource "apple_beta_group" "com_example_app_team" {
  app_id                   = "6478123456"
  name                     = "Team"
  is_internal_group        = true
  has_access_to_all_builds = true
  feedback_enabled         = true
}

import {
  to = apple_beta_group.com_example_app_public_beta
  id = "BG2"
}
resource "apple_beta_group" "com_example_app_public_beta" {
  app_id                    = "6478123456"
  name                      = "Public Beta"
  is_internal_group         = false
  public_link_enabled       = true
  public_link_limit_enabled = true
  public_link_limit         = 500
}

import {
  to = apple_beta_tester.public_beta_ada_example_com
  id = "BG2/ada@example.com"
}
resource "apple_beta_tester" "public_beta_ada_example_com" {
  group_id = apple_beta_group.com_example_app_public_beta.id
  email    = "ada@example.com"
}

import {
  to = apple_beta_app_localization.com_example_app_en_us
  id = "BAL1"
}
resource "apple_beta_app_localization" "com_example_app_en_us" {
  app_id         = "6478123456"
  locale         = "en-US"
  feedback_email = "beta@example.com"
}

import {
  to = apple_beta_app_review_detail.com_example_app
  id = "6478123456"
}
resource "apple_beta_app_review_detail" "com_example_app" {
  app_id                = "6478123456"
  contact_email         = "beta@example.com"
  demo_account_required = false
  demo_account_password = var.beta_review_com_example_app_demo_password
}

