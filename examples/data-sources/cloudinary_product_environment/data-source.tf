# Look up a product environment by its ID...
data "cloudinary_product_environment" "by_id" {
  id = "555asdf0000zxcvb3456qwerty"
}

# ...or by its cloud name.
data "cloudinary_product_environment" "by_cloud_name" {
  cloud_name = "acme-prod"
}

output "cloud_name" {
  value = data.cloudinary_product_environment.by_id.cloud_name
}
