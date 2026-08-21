# Either trigger_id or uri identifies the trigger.
data "cloudinary_trigger" "uploaded" {
  product_environment = "acme-prod"
  uri                 = "https://example.com/webhooks/uploaded"
}

output "event_type" {
  value = data.cloudinary_trigger.uploaded.event_type
}
