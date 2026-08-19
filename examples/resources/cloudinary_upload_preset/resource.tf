# The Admin API authenticates per product environment, so the credentials are
# taken from the environment and its access key rather than from the provider.
locals {
  admin_credentials = {
    cloud_name = cloudinary_product_environment.example.cloud_name
    api_key    = cloudinary_access_key.example.api_key
    api_secret = cloudinary_access_key.example.api_secret
  }
}

# A preset that carries a credential through eval, as the bare value with no
# surrounding JavaScript.
resource "cloudinary_upload_preset" "with_eval" {
  cloud_name = local.admin_credentials.cloud_name
  api_key    = local.admin_credentials.api_key
  api_secret = local.admin_credentials.api_secret

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

# The preset applied to every upload in the environment.
resource "cloudinary_upload_preset" "uploads" {
  cloud_name = local.admin_credentials.cloud_name
  api_key    = local.admin_credentials.api_key
  api_secret = local.admin_credentials.api_secret

  name                                 = "acme-videos"
  unsigned                             = false
  type                                 = "upload"
  asset_folder                         = "acme-prod"
  use_asset_folder_as_public_id_prefix = true
  use_filename                         = false
  unique_filename                      = false
  use_filename_as_display_name         = true
  overwrite                            = true
  invalidate                           = true
  allowed_formats                      = ["mp4", "mov", "webm"]
}
