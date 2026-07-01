data "cloudinary_access_key" "tenant_space" {
  sub_account_id = "555asdf0000zxcvb3456qwerty"
  api_key        = "814814814814814"
}

output "tenant_space_key_name" {
  value = data.cloudinary_access_key.tenant_space.name
}
