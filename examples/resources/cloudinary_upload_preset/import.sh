terraform import cloudinary_upload_preset.uploads acme-prod/acme-videos

# The API key and secret cannot be imported; supply them on the provider.
terraform import cloudinary_upload_preset.with_eval "acme-prod/credential carrier"
