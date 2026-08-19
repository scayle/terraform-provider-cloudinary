terraform {
  required_providers {
    cloudinary = {
      source = "scayle/cloudinary"
    }
  }
}

# Credentials are best supplied via environment variables so they never appear
# in configuration or state inputs:
#   CLOUDINARY_ACCOUNT_ID
#   CLOUDINARY_PROVISIONING_API_KEY
#   CLOUDINARY_PROVISIONING_API_SECRET
provider "cloudinary" {
  # account_id              = "..."          # or CLOUDINARY_ACCOUNT_ID
  # provisioning_api_key    = "..."          # or CLOUDINARY_PROVISIONING_API_KEY (sensitive)
  # provisioning_api_secret = "..."          # or CLOUDINARY_PROVISIONING_API_SECRET (sensitive)
  # api_region              = "api-eu"       # api (default) | api-eu | api-ap

  # Defaults for the Admin API resources (cloudinary_upload_preset,
  # cloudinary_trigger), which authenticate per product environment. Each
  # resource may override them.
  #   CLOUDINARY_CLOUD_NAME
  #   CLOUDINARY_API_KEY
  #   CLOUDINARY_API_SECRET
  # cloud_name              = "..."
  # api_key                 = "..."          # sensitive
  # api_secret              = "..."          # sensitive
}
