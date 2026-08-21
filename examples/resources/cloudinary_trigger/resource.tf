# Notifies an endpoint whenever an asset finishes uploading to this product
# environment.
resource "cloudinary_trigger" "uploaded" {
  product_environment = cloudinary_product_environment.example.id

  uri        = "https://example.com/webhooks/uploaded"
  event_type = "upload"
}
