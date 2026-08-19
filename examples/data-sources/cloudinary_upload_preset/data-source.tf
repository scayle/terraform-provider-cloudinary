data "cloudinary_upload_preset" "uploads" {
  cloud_name = "acme-prod"
  api_key    = "814814814814814"
  api_secret = "..."
  name       = "acme-videos"
}

output "asset_folder" {
  value = data.cloudinary_upload_preset.uploads.asset_folder
}
