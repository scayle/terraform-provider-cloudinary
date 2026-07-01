# Terraform Provider for Cloudinary

A [Terraform](https://www.terraform.io) provider for the [Cloudinary Provisioning API](https://cloudinary.com/documentation/provisioning_api).
It manages **product environments (sub-accounts)** and their **API access keys**, which is the
infrastructure needed to onboard a SCAYLE tenant to Cloudinary (steps 1 and 2 of the tenant
Cloudinary setup):

1. **`cloudinary_sub_account`** – one product environment (sub-account) per tenant.
2. **`cloudinary_access_key`** – an API access key per tenant space (environment).

Matching **data sources** are provided for both, all resources are **importable**, and every
credential field (`api_secret`, `api_access_keys[].secret`) is marked **sensitive** so it is
redacted in `terraform plan`/`apply` output and never leaks into CI logs.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.9
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

# Step 1 – product environment (sub-account)
resource "cloudinary_sub_account" "tenant" {
  name                = "acme-long-tenant-key"
  cloud_name          = "scayle-acme"
  base_sub_account_id = var.base_sub_account_id # EU/US base environment
}

# Step 2 – access key per tenant space
resource "cloudinary_access_key" "tenant_space" {
  sub_account_id = cloudinary_sub_account.tenant.id
  name           = "live"
}

output "cloud_name" {
  value = cloudinary_sub_account.tenant.cloud_name
}

output "api_key" {
  value     = cloudinary_access_key.tenant_space.api_key
  sensitive = true
}

output "api_secret" {
  value     = cloudinary_access_key.tenant_space.api_secret
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

Supplying credentials through environment variables keeps them out of your configuration and
state inputs entirely.

## Importing

```sh
# Sub-account: by product environment ID
terraform import cloudinary_sub_account.tenant <sub_account_id>

# Access key: "<sub_account_id>/<api_key>"
terraform import cloudinary_access_key.tenant_space <sub_account_id>/<api_key>
```

> **Note on secrets:** the Cloudinary Provisioning API only returns an `api_secret` (and the
> sub-account's bootstrap `api_access_keys[].secret`) at creation time. These values cannot be
> recovered on import or refresh; after an import they will be `null` in state.

## Development

```sh
make build      # compile
make test       # unit + integration tests (mocked API, no credentials needed)
make testacc    # acceptance tests (TF_ACC=1, mocked Provisioning API)
make lint       # go vet + gofmt
make generate   # regenerate docs (tfplugindocs)
```

The test suite includes an in-memory mock of the Provisioning API
(`internal/provider/mock_test.go`), so unit, integration, and acceptance tests run without any
Cloudinary credentials. To run acceptance tests against the real API, set the `CLOUDINARY_*`
variables and `TF_ACC=1`.

## License

[MPL-2.0](./LICENSE)
