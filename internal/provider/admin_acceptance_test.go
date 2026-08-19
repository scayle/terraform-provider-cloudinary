package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func adminProviderConfig(provisioningURL, adminURL string) string {
	return fmt.Sprintf(`
provider "cloudinary" {
  account_id              = "acct"
  provisioning_api_key    = "k"
  provisioning_api_secret = "s"
  api_base_url            = %[1]q
  admin_api_base_url      = %[2]q
  cloud_name              = "acme-prod"
  api_key                 = "key"
  api_secret              = "secret"
}
`, provisioningURL, adminURL)
}

func TestAccUploadPresetResource(t *testing.T) {
	provisioning := newMockProvisioning()
	provisioningServer := provisioning.server()
	t.Cleanup(provisioningServer.Close)

	admin := newMockAdmin()
	adminServer := admin.server()
	t.Cleanup(adminServer.Close)

	config := adminProviderConfig(provisioningServer.URL, adminServer.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // Create the generic upload preset, mirroring the deployed acme-prod setup
				Config: config + `
resource "cloudinary_upload_preset" "uploads" {
  cloud_name = "acme-prod"
  api_key    = "key"
  api_secret = "secret"

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
  notification_url                     = "https://example.com/webhooks/uploaded"
  allowed_formats                      = ["mp4", "mov", "webm"]
  context                              = { managed_by = "terraform" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "name", "acme-videos"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "asset_folder", "acme-videos"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "use_asset_folder_as_public_id_prefix", "true"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "use_filename", "false"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "allowed_formats.#", "3"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "allowed_formats.0", "mp4"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "context.managed_by", "terraform"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "id", "acme-prod/acme-videos"),
				),
			},
			{ // Update
				Config: config + `
resource "cloudinary_upload_preset" "uploads" {
  cloud_name = "acme-prod"
  api_key    = "key"
  api_secret = "secret"

  name            = "acme-videos"
  type            = "upload"
  asset_folder    = "acme-videos-renamed"
  overwrite       = false
  allowed_formats = ["mp4"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "asset_folder", "acme-videos-renamed"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "overwrite", "false"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.uploads", "allowed_formats.#", "1"),
					resource.TestCheckNoResourceAttr("cloudinary_upload_preset.uploads", "notification_url"),
				),
			},
		},
	})
}

// eval may carry a credential, so it must round-trip untouched.
func TestAccUploadPresetEval(t *testing.T) {
	provisioning := newMockProvisioning()
	provisioningServer := provisioning.server()
	t.Cleanup(provisioningServer.Close)

	admin := newMockAdmin()
	adminServer := admin.server()
	t.Cleanup(adminServer.Close)

	config := adminProviderConfig(provisioningServer.URL, adminServer.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config + `
resource "cloudinary_upload_preset" "with_eval" {
  cloud_name = "acme-prod"
  api_key    = "key"
  api_secret = "secret"

  name                                 = "credential carrier"
  unsigned                             = false
  type                                 = "upload"
  use_filename                         = false
  unique_filename                      = false
  use_filename_as_display_name         = true
  use_asset_folder_as_public_id_prefix = false
  overwrite                            = true
  eval                                 = "example-eval-value"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cloudinary_upload_preset.with_eval", "name", "credential carrier"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.with_eval", "eval",
						"example-eval-value"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.with_eval", "use_asset_folder_as_public_id_prefix", "false"),
					resource.TestCheckResourceAttr("cloudinary_upload_preset.with_eval", "id", "acme-prod/credential carrier"),
				),
			},
			{ // Import by "<cloud_name>/<name>"
				ResourceName:            "cloudinary_upload_preset.with_eval",
				ImportState:             true,
				ImportStateId:           "acme-prod/credential carrier",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"api_key", "api_secret"},
			},
		},
	})
}

func TestAccTriggerResource(t *testing.T) {
	provisioning := newMockProvisioning()
	provisioningServer := provisioning.server()
	t.Cleanup(provisioningServer.Close)

	admin := newMockAdmin()
	adminServer := admin.server()
	t.Cleanup(adminServer.Close)

	config := adminProviderConfig(provisioningServer.URL, adminServer.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // Create
				Config: config + `
resource "cloudinary_trigger" "video_uploaded" {
  cloud_name = "acme-prod"
  api_key    = "key"
  api_secret = "secret"

  uri        = "https://example.com/webhooks/uploaded"
  event_type = "upload"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cloudinary_trigger.video_uploaded", "event_type", "upload"),
					resource.TestCheckResourceAttr("cloudinary_trigger.video_uploaded", "uri",
						"https://example.com/webhooks/uploaded"),
					resource.TestCheckResourceAttrSet("cloudinary_trigger.video_uploaded", "trigger_id"),
					resource.TestCheckResourceAttrSet("cloudinary_trigger.video_uploaded", "created_at"),
				),
			},
			{ // Update
				Config: config + `
resource "cloudinary_trigger" "video_uploaded" {
  cloud_name = "acme-prod"
  api_key    = "key"
  api_secret = "secret"

  uri        = "https://example.com/webhooks/uploaded-v2"
  event_type = "upload"
  additive   = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cloudinary_trigger.video_uploaded", "uri",
						"https://example.com/webhooks/uploaded-v2"),
					resource.TestCheckResourceAttr("cloudinary_trigger.video_uploaded", "additive", "true"),
				),
			},
		},
	})
}

func TestAccUploadPresetDataSource(t *testing.T) {
	provisioning := newMockProvisioning()
	provisioningServer := provisioning.server()
	t.Cleanup(provisioningServer.Close)

	admin := newMockAdmin()
	adminServer := admin.server()
	t.Cleanup(adminServer.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: adminProviderConfig(provisioningServer.URL, adminServer.URL) + `
resource "cloudinary_upload_preset" "uploads" {
  cloud_name   = "acme-prod"
  api_key      = "key"
  api_secret   = "secret"
  name         = "acme-videos"
  asset_folder = "acme-videos"
}

data "cloudinary_upload_preset" "uploads" {
  cloud_name = "acme-prod"
  api_key    = "key"
  api_secret = "secret"
  name       = cloudinary_upload_preset.uploads.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cloudinary_upload_preset.uploads", "name", "acme-videos"),
					resource.TestCheckResourceAttr("data.cloudinary_upload_preset.uploads", "asset_folder", "acme-videos"),
				),
			},
		},
	})
}

func TestAccTriggerDataSource(t *testing.T) {
	provisioning := newMockProvisioning()
	provisioningServer := provisioning.server()
	t.Cleanup(provisioningServer.Close)

	admin := newMockAdmin()
	adminServer := admin.server()
	t.Cleanup(adminServer.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: adminProviderConfig(provisioningServer.URL, adminServer.URL) + `
resource "cloudinary_trigger" "video_uploaded" {
  cloud_name = "acme-prod"
  api_key    = "key"
  api_secret = "secret"
  uri        = "https://example.com/webhooks/uploaded"
  event_type = "upload"
}

data "cloudinary_trigger" "video_uploaded" {
  cloud_name = "acme-prod"
  api_key    = "key"
  api_secret = "secret"
  trigger_id = cloudinary_trigger.video_uploaded.trigger_id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cloudinary_trigger.video_uploaded", "event_type", "upload"),
					resource.TestCheckResourceAttr("data.cloudinary_trigger.video_uploaded", "uri",
						"https://example.com/webhooks/uploaded"),
				),
			},
		},
	})
}
