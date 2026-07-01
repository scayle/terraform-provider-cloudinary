# Look up a sub-account by its ID...
data "cloudinary_sub_account" "by_id" {
  id = "555asdf0000zxcvb3456qwerty"
}

# ...or by its cloud name.
data "cloudinary_sub_account" "by_cloud_name" {
  cloud_name = "scayle-acme"
}

output "tenant_cloud_name" {
  value = data.cloudinary_sub_account.by_id.cloud_name
}
