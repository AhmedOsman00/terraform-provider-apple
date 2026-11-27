variable "app_review_com_example_app_ios_demo_password" {
  type        = string
  sensitive   = true
  default     = null
  description = "Apple does not return this password, so the export could not. Left null, Terraform leaves the one Apple holds alone."
}

variable "beta_review_com_example_app_demo_password" {
  type        = string
  sensitive   = true
  default     = null
  description = "Apple does not return this password, so the export could not. Left null, Terraform leaves the one Apple holds alone."
}
