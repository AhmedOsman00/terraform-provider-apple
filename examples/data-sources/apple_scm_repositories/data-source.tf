data "apple_scm_repositories" "all" {}

output "repositories" {
  value = data.apple_scm_repositories.all.repositories
}
