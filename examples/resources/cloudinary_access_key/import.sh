# Access keys can be imported as "<sub_account_id>/<api_key>"...
terraform import cloudinary_access_key.tenant_space 555asdf0000zxcvb3456qwerty/814814814814814

# ...or by their human names as "<cloud_name>/<key_name>".
terraform import cloudinary_access_key.tenant_space scayle-acme/live

# Note: the API secret is only returned at creation time and cannot be recovered
# on import; it will be null in state afterwards.
