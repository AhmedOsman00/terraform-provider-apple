data "apple_ci_mac_os_versions" "all" {}

output "versions" {
  value = data.apple_ci_mac_os_versions.all.versions
}
