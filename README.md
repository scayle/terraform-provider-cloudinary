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
| `cloud_name`              | `CLOUDINARY_CLOUD_NAME`              | Admin API escape hatch: cloud name.          |
| `api_key`                 | `CLOUDINARY_API_KEY`                 | Admin API escape hatch: key (sensitive).     |
| `api_secret`              | `CLOUDINARY_API_SECRET`              | Admin API escape hatch: secret (sensitive).  |
| `admin_api_base_url`      | `CLOUDINARY_ADMIN_API_BASE_URL`      | Override the Admin API base URL (proxy/test). |

Supplying credentials through environment variables keeps them out of your configuration and
state inputs entirely.

### Admin API credentials

The Provisioning API authenticates once at account level, but the Admin API authenticates **per
product environment**. Rather than asking you to supply those credentials, `cloudinary_upload_preset`
and `cloudinary_trigger` reference a `product_environment` by ID or cloud name, and the provider
resolves its credentials through the Provisioning API. No secret appears in your configuration,
plan, or their state:

```hcl
resource "cloudinary_product_environment" "example" {
  name       = "acme-prod"
  cloud_name = "acme-prod"
}

resource "cloudinary_upload_preset" "uploads" {
  product_environment = cloudinary_product_environment.example.id # unknown at plan time, which is fine
  name                = "acme-videos"
  asset_folder        = "acme-videos"
}
```

Resource arguments may be unknown at plan time — only *provider* configuration must resolve then —
so this creates the environment, resolves its credentials and creates the preset in a single apply.

By default the oldest enabled access key of the environment is used. Set `access_key` to a key name
to pin it, which keeps key rotation from touching every preset:

```hcl
resource "cloudinary_access_key" "terraform" {
  sub_account_id = cloudinary_product_environment.example.id
  name           = "terraform"
}

resource "cloudinary_upload_preset" "uploads" {
  product_environment = cloudinary_product_environment.example.id
  access_key          = cloudinary_access_key.terraform.name # a name, not a secret
  name                = "acme-videos"
}
```

Resolved credentials are cached in memory for the lifetime of the process, never in state. The
provider-level `cloud_name` / `api_key` / `api_secret` remain available as an escape hatch for users
who hold product environment credentials but no provisioning credentials.

### Parameters Cloudinary may ignore

Some upload parameters depend on an add-on being enabled for the product environment — `auto_tagging`,
`categorization`, `background_removal`, `detection`, `ocr` and `moderation` among them. When the add-on
is not enabled Cloudinary accepts the request but silently discards the parameter.

The provider re-reads the preset after writing it and raises a warning naming any parameter that was
not stored. Because the desired state genuinely was not reached, the parameter keeps showing as a diff
until the add-on is enabled or the parameter is removed from the configuration.

## Importing

```sh
# Product environment: by ID or cloud name
terraform import cloudinary_product_environment.example <id_or_cloud_name>

# Access key: "<sub_account_id>/<api_key>" or "<cloud_name>/<key_name>"
terraform import cloudinary_access_key.example <sub_account_id>/<api_key>

# Upload preset: "<product_environment>/<name>"
terraform import cloudinary_upload_preset.example <cloud_name_or_id>/<name>

# Trigger: "<product_environment>/<trigger_id>"
terraform import cloudinary_trigger.example <cloud_name_or_id>/<trigger_id>
```

> **Note on secrets:** a product environment's `initial_access_key.secret` is only returned when the
> environment is created, so it is `null` after an import. Access key secrets, by contrast, are
> returned by the list endpoint and survive import and refresh.

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
