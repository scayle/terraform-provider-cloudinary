package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/operations"
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
// Provisioning API.
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

// adminResolver derives per-product-environment credentials from the
// account-level provisioning credentials, so configurations never have to carry
// an api_secret. Resolved credentials are cached in memory for the process
// lifetime; they are deliberately not written to private state, which is
// serialised into the state file.
type adminResolver struct {
	provisioning *cldprovisioning.CldProvisioning
	defaults     adminConfig

	mu    sync.Mutex
	cache map[string]adminConfig
}

func newAdminResolver(provisioning *cldprovisioning.CldProvisioning, defaults adminConfig) *adminResolver {
	return &adminResolver{
		provisioning: provisioning,
		defaults:     defaults,
		cache:        map[string]adminConfig{},
	}
}

// clientFor resolves credentials for the referenced product environment. An
// empty environment reference falls back to the provider-level credentials, for
// users who hold environment credentials but no provisioning ones.
func (r *adminResolver) clientFor(ctx context.Context, environment, accessKey string, diags *diag.Diagnostics) (*admin.API, adminConfig) {
	if environment == "" {
		if !r.defaults.complete() {
			diags.AddAttributeError(
				path.Root("product_environment"),
				"Missing Cloudinary Admin API Credentials",
				"Set product_environment so the provider can resolve credentials from the Provisioning API, or "+
					"configure cloud_name, api_key and api_secret on the provider.",
			)
			return nil, adminConfig{}
		}
		return r.build(r.defaults, diags)
	}

	cacheKey := environment + "/" + accessKey

	r.mu.Lock()
	cached, ok := r.cache[cacheKey]
	r.mu.Unlock()
	if ok {
		return r.build(cached, diags)
	}

	cfg, err := r.resolve(ctx, environment, accessKey)
	if err != nil {
		diags.AddError("Error resolving Cloudinary Admin API credentials", err.Error())
		return nil, adminConfig{}
	}

	r.mu.Lock()
	r.cache[cacheKey] = cfg
	r.mu.Unlock()

	return r.build(cfg, diags)
}

func (r *adminResolver) resolve(ctx context.Context, environment, accessKey string) (adminConfig, error) {
	if r.provisioning == nil {
		return adminConfig{}, errors.New("the provider has no provisioning credentials configured")
	}

	env, err := resolveProductEnvironment(ctx, r.provisioning, environment)
	if err != nil {
		return adminConfig{}, err
	}

	res, err := r.provisioning.AccessKeys.List(ctx, operations.GetAccessKeysRequest{SubAccountID: deref(env.ID)})
	if err != nil {
		return adminConfig{}, err
	}

	key := pickAccessKey(res.AccessKeys, accessKey)
	if key == nil {
		if accessKey != "" {
			return adminConfig{}, fmt.Errorf("no access key named %q in product environment %q", accessKey, environment)
		}
		return adminConfig{}, fmt.Errorf("product environment %q has no enabled access key", environment)
	}
	if deref(key.APISecret) == "" {
		return adminConfig{}, fmt.Errorf("the Provisioning API did not return a secret for access key %q", deref(key.APIKey))
	}

	return adminConfig{
		CloudName: deref(env.CloudName),
		APIKey:    deref(key.APIKey),
		APISecret: deref(key.APISecret),
		BaseURL:   r.defaults.BaseURL,
	}, nil
}

func (r *adminResolver) build(cfg adminConfig, diags *diag.Diagnostics) (*admin.API, adminConfig) {
	client, err := newAdminAPI(cfg)
	if err != nil {
		diags.AddError("Error configuring Cloudinary Admin API client", err.Error())
		return nil, cfg
	}
	return client, cfg
}

// The product environment is referenced by ID or cloud name rather than by
// credentials: a provider instance configured from
// cloudinary_access_key.*.api_secret would be unknown at plan time on a first
// apply, whereas an unknown resource argument is resolved during apply.
func adminReferenceAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"product_environment": schema.StringAttribute{
			Required: true,
			MarkdownDescription: "The ID or cloud name of the product environment to act on. The provider resolves " +
				"the Admin API credentials for it through the Provisioning API. Changing it forces a new resource.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"access_key": schema.StringAttribute{
			Optional: true,
			MarkdownDescription: "The name of the access key to authenticate with. Defaults to the oldest enabled " +
				"key of the product environment. Pin it to keep key rotation from touching every resource.",
		},
	}
}

func adminReferenceDataSourceAttributes() map[string]dsschema.Attribute {
	return map[string]dsschema.Attribute{
		"product_environment": dsschema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The ID or cloud name of the product environment to read from.",
		},
		"access_key": dsschema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "The name of the access key to authenticate with. Defaults to the oldest enabled key.",
		},
	}
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

func adminReference(ctx context.Context, src attributeGetter, diags *diag.Diagnostics) (string, string) {
	var environment, accessKey types.String
	diags.Append(src.GetAttribute(ctx, path.Root("product_environment"), &environment)...)
	diags.Append(src.GetAttribute(ctx, path.Root("access_key"), &accessKey)...)
	return environment.ValueString(), accessKey.ValueString()
}
