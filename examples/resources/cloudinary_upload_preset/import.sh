# "<product_environment>/<name>", where the environment is an ID or a cloud name.
terraform import cloudinary_upload_preset.uploads acme-prod/acme-videos

terraform import cloudinary_upload_preset.with_eval "acme-prod/credential carrier"
