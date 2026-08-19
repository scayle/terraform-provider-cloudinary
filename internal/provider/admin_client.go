package provider

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"github.com/cloudinary/cloudinary-go/v2/config"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// adminConfig holds the credentials of a single product environment. The Admin
// API authenticates per product environment, unlike the account-level
// Provisioning API, so these travel with each resource.
type adminConfig struct {
	CloudName string
	APIKey    string
	APISecret string
	BaseURL   string
}

func (c adminConfig) complete() bool {
	return c.CloudName != "" && c.APIKey != "" && c.APISecret != ""
}

func newAdminAPI(cfg adminConfig) (*admin.API, error) {
	conf, err := config.NewFromParams(cfg.CloudName, cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, err
	}
	if cfg.BaseURL != "" {
		conf.API.UploadPrefix = cfg.BaseURL
	}
	return admin.NewWithConfiguration(conf)
}

// Credentials sit on the resource rather than on a second, aliased provider
// instance: a provider configured from cloudinary_access_key.*.api_secret is
// unknown at plan time on a first apply, which Terraform rejects.
func adminCredentialAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"cloud_name": schema.StringAttribute{
			Optional: true,
			MarkdownDescription: "The cloud name of the product environment to act on. Defaults to the provider's " +
				"`cloud_name`. Changing it forces a new resource.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"api_key": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "The API key of that product environment. Defaults to the provider's `api_key`.",
		},
		"api_secret": schema.StringAttribute{
			Optional:            true,
			Sensitive:           true,
			MarkdownDescription: "The API secret of that product environment. Defaults to the provider's `api_secret`.",
		},
	}
}

func adminCredentialDataSourceAttributes() map[string]dsschema.Attribute {
	return map[string]dsschema.Attribute{
		"cloud_name": dsschema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "The cloud name of the product environment to read from. Defaults to the provider's `cloud_name`.",
		},
		"api_key": dsschema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "The API key of that product environment. Defaults to the provider's `api_key`.",
		},
		"api_secret": dsschema.StringAttribute{
			Optional:            true,
			Sensitive:           true,
			MarkdownDescription: "The API secret of that product environment. Defaults to the provider's `api_secret`.",
		},
	}
}

func resolveAdminAPI(defaults adminConfig, cloudName, apiKey, apiSecret types.String, diags *diag.Diagnostics) (*admin.API, adminConfig) {
	cfg := adminConfig{
		CloudName: firstNonEmpty(cloudName, defaults.CloudName),
		APIKey:    firstNonEmpty(apiKey, defaults.APIKey),
		APISecret: firstNonEmpty(apiSecret, defaults.APISecret),
		BaseURL:   defaults.BaseURL,
	}

	if !cfg.complete() {
		for _, attr := range []struct{ name, value string }{
			{"cloud_name", cfg.CloudName},
			{"api_key", cfg.APIKey},
			{"api_secret", cfg.APISecret},
		} {
			if attr.value == "" {
				diags.AddAttributeError(
					path.Root(attr.name),
					"Missing Cloudinary Admin API Credentials",
					fmt.Sprintf("Set %q on this resource or on the provider. The Admin API authenticates per "+
						"product environment and cannot reuse the account-level provisioning credentials.", attr.name),
				)
			}
		}
		return nil, cfg
	}

	client, err := newAdminAPI(cfg)
	if err != nil {
		diags.AddError("Error configuring Cloudinary Admin API client", err.Error())
		return nil, cfg
	}
	return client, cfg
}

type adminAPIError struct {
	message string
}

func (e *adminAPIError) Error() string { return e.message }

// The Admin API reports failures in the response body; the SDK's returned error
// only covers transport-level problems.
func adminError(resp api.ErrorResp) error {
	if resp.Message == "" {
		return nil
	}
	return &adminAPIError{message: resp.Message}
}

func isAdminNotFound(err error) bool {
	var apiErr *adminAPIError
	if errors.As(err, &apiErr) {
		msg := strings.ToLower(apiErr.message)
		return strings.Contains(msg, "not found") || strings.Contains(msg, "can't find")
	}
	return isNotFound(err)
}
