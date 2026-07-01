package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccProtoV6ProviderFactories instantiates the provider for acceptance tests.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"cloudinary": providerserver.NewProtocol6WithError(New("test")()),
}

// providerConfig returns an HCL provider block pointed at the mock server with
// dummy (mock-accepted) credentials.
func providerConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "cloudinary" {
  account_id              = "acct"
  provisioning_api_key    = "k"
  provisioning_api_secret = "s"
  api_base_url            = %[1]q
}
`, baseURL)
}

func TestAccSubAccountResource(t *testing.T) {
	mock := newMockProvisioning()
	ts := mock.server()
	t.Cleanup(ts.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // Create
				Config: providerConfig(ts.URL) + `
resource "cloudinary_sub_account" "test" {
  name       = "acme"
  cloud_name = "scayle-acme"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cloudinary_sub_account.test", "name", "acme"),
					resource.TestCheckResourceAttr("cloudinary_sub_account.test", "cloud_name", "scayle-acme"),
					resource.TestCheckResourceAttr("cloudinary_sub_account.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("cloudinary_sub_account.test", "id"),
					resource.TestCheckResourceAttrSet("cloudinary_sub_account.test", "initial_access_key.key"),
					resource.TestCheckResourceAttrSet("cloudinary_sub_account.test", "initial_access_key.secret"),
				),
			},
			{ // Import by cloud name (initial_access_key is unrecoverable on import)
				ResourceName:            "cloudinary_sub_account.test",
				ImportState:             true,
				ImportStateId:           "scayle-acme",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"initial_access_key"},
			},
			{ // Update
				Config: providerConfig(ts.URL) + `
resource "cloudinary_sub_account" "test" {
  name       = "acme-renamed"
  cloud_name = "scayle-acme"
  enabled    = false
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cloudinary_sub_account.test", "name", "acme-renamed"),
					resource.TestCheckResourceAttr("cloudinary_sub_account.test", "enabled", "false"),
				),
			},
		},
	})
}

func TestAccAccessKeyResource(t *testing.T) {
	mock := newMockProvisioning()
	ts := mock.server()
	t.Cleanup(ts.Close)

	config := providerConfig(ts.URL) + `
resource "cloudinary_sub_account" "test" {
  name       = "acme"
  cloud_name = "scayle-acme"
}

resource "cloudinary_access_key" "test" {
  sub_account_id = cloudinary_sub_account.test.id
  name           = "live"
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ // Create
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("cloudinary_access_key.test", "name", "live"),
					resource.TestCheckResourceAttr("cloudinary_access_key.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("cloudinary_access_key.test", "api_key"),
					resource.TestCheckResourceAttrSet("cloudinary_access_key.test", "api_secret"),
					resource.TestCheckResourceAttrSet("cloudinary_access_key.test", "id"),
				),
			},
			{ // Import by "<cloud_name>/<key_name>" (the secret cannot be recovered)
				ResourceName:            "cloudinary_access_key.test",
				ImportState:             true,
				ImportStateId:           "scayle-acme/live",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"api_secret"},
			},
		},
	})
}

func TestAccSubAccountDataSource(t *testing.T) {
	mock := newMockProvisioning()
	ts := mock.server()
	t.Cleanup(ts.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(ts.URL) + `
resource "cloudinary_sub_account" "test" {
  name       = "acme"
  cloud_name = "scayle-acme"
}

data "cloudinary_sub_account" "test" {
  id = cloudinary_sub_account.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cloudinary_sub_account.test", "name", "acme"),
					resource.TestCheckResourceAttr("data.cloudinary_sub_account.test", "cloud_name", "scayle-acme"),
				),
			},
		},
	})
}

func TestAccSubAccountDataSourceByCloudName(t *testing.T) {
	mock := newMockProvisioning()
	ts := mock.server()
	t.Cleanup(ts.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(ts.URL) + `
resource "cloudinary_sub_account" "test" {
  name       = "acme"
  cloud_name = "scayle-acme"
}

data "cloudinary_sub_account" "by_cloud_name" {
  cloud_name = cloudinary_sub_account.test.cloud_name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cloudinary_sub_account.by_cloud_name", "name", "acme"),
					resource.TestCheckResourceAttr("data.cloudinary_sub_account.by_cloud_name", "cloud_name", "scayle-acme"),
					resource.TestCheckResourceAttrPair(
						"data.cloudinary_sub_account.by_cloud_name", "id",
						"cloudinary_sub_account.test", "id",
					),
				),
			},
		},
	})
}

func TestAccAccessKeyDataSource(t *testing.T) {
	mock := newMockProvisioning()
	ts := mock.server()
	t.Cleanup(ts.Close)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(ts.URL) + `
resource "cloudinary_sub_account" "test" {
  name       = "acme"
  cloud_name = "scayle-acme"
}

resource "cloudinary_access_key" "test" {
  sub_account_id = cloudinary_sub_account.test.id
  name           = "live"
}

data "cloudinary_access_key" "test" {
  sub_account_id = cloudinary_sub_account.test.id
  api_key        = cloudinary_access_key.test.api_key
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.cloudinary_access_key.test", "name", "live"),
					resource.TestCheckResourceAttr("data.cloudinary_access_key.test", "enabled", "true"),
				),
			},
		},
	})
}
