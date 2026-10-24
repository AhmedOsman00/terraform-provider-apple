data "apple_ci_products" "all" {}

output "products" {
  value = data.apple_ci_products.all.products
}
