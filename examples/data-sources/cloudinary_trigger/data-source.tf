# Either trigger_id or uri identifies the trigger.
data "cloudinary_trigger" "uploaded" {
  cloud_name = "acme-prod"
  api_key    = "814814814814814"
  api_secret = "..."
  uri        = "https://example.com/webhooks/uploaded"
}

output "event_type" {
  value = data.cloudinary_trigger.uploaded.event_type
}
