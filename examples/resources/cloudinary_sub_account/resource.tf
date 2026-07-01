# Step 1: one product environment (sub-account) per SCAYLE tenant.
resource "cloudinary_sub_account" "tenant" {
  name       = "acme-long-tenant-key"
  cloud_name = "scayle-acme"

  # The EU/US base environment to copy settings from (create-time only).
  base_sub_account_id = var.base_sub_account_id
}

# The auto-provisioned access key and its (sensitive) secret.
output "tenant_cloud_name" {
  value = cloudinary_sub_account.tenant.cloud_name
}

output "tenant_initial_api_key" {
  value     = cloudinary_sub_account.tenant.initial_access_key.key
  sensitive = true
}

output "tenant_initial_api_secret" {
  value     = cloudinary_sub_account.tenant.initial_access_key.secret
  sensitive = true
}
