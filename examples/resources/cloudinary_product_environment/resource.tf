resource "cloudinary_product_environment" "example" {
  name       = "Example Production"
  cloud_name = "acme-prod"

  # Optionally copy settings from an existing product environment (create-time only).
  # base_sub_account_id = "555asdf0000zxcvb3456qwerty"
}

output "cloud_name" {
  value = cloudinary_product_environment.example.cloud_name
}

# The access key auto-provisioned with the product environment. Its secret is
# only available at creation time.
output "initial_api_key" {
  value     = cloudinary_product_environment.example.initial_access_key.key
  sensitive = true
}

output "initial_api_secret" {
  value     = cloudinary_product_environment.example.initial_access_key.secret
  sensitive = true
}
