# Terraform Provider for Cloudinary

A [Terraform](https://www.terraform.io) provider for the [Cloudinary Provisioning API](https://cloudinary.com/documentation/provisioning_api)
and [Admin API](https://cloudinary.com/documentation/admin_api). It manages Cloudinary **product
environments** (previously known as sub-accounts), their **API access keys**, and the **upload
presets** and **triggers** within them:

- **`cloudinary_product_environment`** – a product environment within a Cloudinary account.
- **`cloudinary_access_key`** – an API access key within a product environment.
- **`cloudinary_upload_preset`** – an upload preset within a product environment.
- **`cloudinary_trigger`** – a webhook trigger within a product environment.

Matching **data sources** are provided for both, all resources are **importable**, and every
credential field (`api_secret`, the product environment's `initial_access_key.secret`) is marked
**sensitive** so it is redacted in `terraform plan`/`apply` output and never leaks into CI logs.

> **Note:** the Provisioning API still uses the legacy term "sub-account" in its endpoints and
> fields (e.g. `sub_account_id`), so that attribute name is retained where it mirrors the API.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.13
- [Go](https://go.dev/doc/install) >= 1.24 (to build the provider)
- A Cloudinary account with **provisioning (account management) API** credentials.

## Using the provider

```hcl
terraform {
  required_providers {
    cloudinary = {
      source = "scayle/cloudinary"
    }
  }
}

provider "cloudinary" {
  # Prefer environment variables for credentials (see below).
  # account_id              = "..."
  # provisioning_api_key    = "..."   # sensitive
  # provisioning_api_secret = "..."   # sensitive
  # api_region              = "api-eu" # api (default) | api-eu | api-ap
}

resource "cloudinary_product_environment" "example" {
  name       = "Example Production"
  cloud_name = "acme-prod"
}

resource "cloudinary_access_key" "example" {
  sub_account_id = cloudinary_product_environment.example.id
  name           = "primary"
}

output "cloud_name" {
  value = cloudinary_product_environment.example.cloud_name
}

output "api_key" {
  value     = cloudinary_access_key.example.api_key
  sensitive = true
}

output "api_secret" {
  value     = cloudinary_access_key.example.api_secret
  sensitive = true
}
```

### Configuration

The provider accepts configuration via attributes or environment variables (attributes win):

| Attribute                 | Environment variable                 | Description                                  |
| ------------------------- | ------------------------------------ | -------------------------------------------- |
| `account_id`              | `CLOUDINARY_ACCOUNT_ID`              | Cloudinary account ID.                       |
| `provisioning_api_key`    | `CLOUDINARY_PROVISIONING_API_KEY`    | Provisioning API key (sensitive).            |
| `provisioning_api_secret` | `CLOUDINARY_PROVISIONING_API_SECRET` | Provisioning API secret (sensitive).         |
| `api_region`              | `CLOUDINARY_API_REGION`              | `api` (default), `api-eu`, or `api-ap`.      |
| `api_base_url`            | `CLOUDINARY_API_BASE_URL`            | Override the full API base URL (proxy/test). |
| `cloud_name`              | `CLOUDINARY_CLOUD_NAME`              | Default cloud name for Admin API resources.  |
| `api_key`                 | `CLOUDINARY_API_KEY`                 | Default product environment key (sensitive). |
| `api_secret`              | `CLOUDINARY_API_SECRET`              | Default product environment secret (sensitive). |
| `admin_api_base_url`      | `CLOUDINARY_ADMIN_API_BASE_URL`      | Override the Admin API base URL (proxy/test). |

Supplying credentials through environment variables keeps them out of your configuration and
state inputs entirely.

### Admin API credentials

The Provisioning API authenticates once at account level, but the Admin API authenticates **per
product environment**. `cloudinary_upload_preset` and `cloudinary_trigger` therefore take their own
`cloud_name`, `api_key` and `api_secret`, falling back to the provider-level defaults above.

They are attributes on the resource rather than a second, aliased provider block on purpose: a
provider configured from `cloudinary_access_key.example.api_secret` would be unknown at plan time
on a first apply, which Terraform rejects. Keeping them on the resource allows a product
environment, its access key and its presets to be created in a single apply:

```hcl
resource "cloudinary_upload_preset" "example" {
  cloud_name = cloudinary_product_environment.example.cloud_name
  api_key    = cloudinary_access_key.example.api_key
  api_secret = cloudinary_access_key.example.api_secret

  name         = "acme-videos"
  asset_folder = "acme-videos"
}
```

## Importing

```sh
# Product environment: by ID or cloud name
terraform import cloudinary_product_environment.example <id_or_cloud_name>

# Access key: "<sub_account_id>/<api_key>" or "<cloud_name>/<key_name>"
terraform import cloudinary_access_key.example <sub_account_id>/<api_key>

# Upload preset: "<cloud_name>/<name>"
terraform import cloudinary_upload_preset.example <cloud_name>/<name>

# Trigger: "<cloud_name>/<trigger_id>"
terraform import cloudinary_trigger.example <cloud_name>/<trigger_id>
```

> **Note on secrets:** the Cloudinary Provisioning API only returns an `api_secret` (and a product
> environment's `initial_access_key.secret`) at creation time. These values cannot be recovered on
> import or refresh; after an import they will be `null` in state.

## Development

```sh
make build      # compile
make test       # unit + integration tests (mocked API, no credentials needed)
make testacc    # acceptance tests (TF_ACC=1, mocked Provisioning API)
make lint       # go vet + gofmt
make generate   # regenerate docs (tfplugindocs)
```

The test suite includes in-memory mocks of the Provisioning API
(`internal/provider/mock_test.go`) and the Admin API (`internal/provider/mock_admin_test.go`), so
unit, integration, and acceptance tests run without any Cloudinary credentials. To run acceptance tests against the real API, set the `CLOUDINARY_*`
variables and `TF_ACC=1`.

## License

[MPL-2.0](./LICENSE)
