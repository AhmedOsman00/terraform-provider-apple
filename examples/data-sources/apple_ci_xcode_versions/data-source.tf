data "apple_ci_xcode_versions" "all" {}

output "versions" {
  value = data.apple_ci_xcode_versions.all.versions
}
