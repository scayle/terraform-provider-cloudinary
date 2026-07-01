# Step 2: an access key per tenant space (environment) within a sub-account.
resource "cloudinary_access_key" "tenant_space" {
  sub_account_id = cloudinary_sub_account.tenant.id
  name           = "live"
  enabled        = true
}

output "tenant_space_api_key" {
  value     = cloudinary_access_key.tenant_space.api_key
  sensitive = true
}

output "tenant_space_api_secret" {
  value     = cloudinary_access_key.tenant_space.api_secret
  sensitive = true
}
