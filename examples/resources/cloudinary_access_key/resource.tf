resource "cloudinary_access_key" "example" {
  sub_account_id = cloudinary_product_environment.example.id
  name           = "primary"
  enabled        = true
}

output "api_key" {
  value     = cloudinary_access_key.example.api_key
  sensitive = true
}

output "api_secret" {
  value     = cloudinary_access_key.example.api_secret
  sensitive = true
}
