data "cloudinary_upload_preset" "uploads" {
  product_environment = "acme-prod"
  name                = "acme-videos"
}

output "asset_folder" {
  value = data.cloudinary_upload_preset.uploads.asset_folder
}
