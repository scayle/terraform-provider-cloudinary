# Notifies an endpoint whenever an asset finishes uploading to this product
# environment.
resource "cloudinary_trigger" "uploaded" {
  cloud_name = cloudinary_product_environment.example.cloud_name
  api_key    = cloudinary_access_key.example.api_key
  api_secret = cloudinary_access_key.example.api_secret

  uri        = "https://example.com/webhooks/uploaded"
  event_type = "upload"
}
