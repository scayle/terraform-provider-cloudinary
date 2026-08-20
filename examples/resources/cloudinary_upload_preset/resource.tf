# The provider resolves the product environment's Admin API credentials through
# the Provisioning API, so no secret appears in configuration or state.
resource "cloudinary_upload_preset" "uploads" {
  product_environment = cloudinary_product_environment.example.id

  name                                 = "acme-videos"
  unsigned                             = false
  type                                 = "upload"
  asset_folder                         = "acme-videos"
  use_asset_folder_as_public_id_prefix = true
  use_filename                         = false
  unique_filename                      = false
  use_filename_as_display_name         = true
  overwrite                            = true
  invalidate                           = true
  allowed_formats                      = ["mp4", "mov", "webm"]
}

# eval can carry a credential into a preset as a bare value, so it is sensitive.
# Generate the value with Terraform rather than committing it.
resource "random_password" "example" {
  length  = 64
  special = false
  upper   = true
  numeric = true
}

resource "cloudinary_upload_preset" "with_eval" {
  product_environment = cloudinary_product_environment.example.id

  # Pin the key so rotating other keys does not touch this preset.
  access_key = cloudinary_access_key.terraform.name

  name                                 = "credential carrier"
  unsigned                             = false
  type                                 = "upload"
  use_filename                         = false
  unique_filename                      = false
  use_filename_as_display_name         = true
  use_asset_folder_as_public_id_prefix = false
  overwrite                            = true
  eval                                 = random_password.example.result
}
