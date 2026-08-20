package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Environment variables used as fallback for provider configuration.
const (
	envAccountID = "CLOUDINARY_ACCOUNT_ID"
	envAPIKey    = "CLOUDINARY_PROVISIONING_API_KEY"
	envAPISecret = "CLOUDINARY_PROVISIONING_API_SECRET"
	envRegion    = "CLOUDINARY_API_REGION"
	envBaseURL   = "CLOUDINARY_API_BASE_URL"

	envCloudName      = "CLOUDINARY_CLOUD_NAME"
	envAdminAPIKey    = "CLOUDINARY_API_KEY"
	envAdminAPISecret = "CLOUDINARY_API_SECRET"
	envAdminBaseURL   = "CLOUDINARY_ADMIN_API_BASE_URL"
)

// Ensure cloudinaryProvider satisfies the provider.Provider interface.
var _ provider.Provider = (*cloudinaryProvider)(nil)

// cloudinaryProvider implements the Cloudinary Provisioning API provider.
type cloudinaryProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" during acceptance tests.
	version string
}

// cloudinaryProviderModel maps provider schema data to a Go type.
type cloudinaryProviderModel struct {
	AccountID             types.String `tfsdk:"account_id"`
	ProvisioningAPIKey    types.String `tfsdk:"provisioning_api_key"`
	ProvisioningAPISecret types.String `tfsdk:"provisioning_api_secret"`
	APIRegion             types.String `tfsdk:"api_region"`
	APIBaseURL            types.String `tfsdk:"api_base_url"`
	CloudName             types.String `tfsdk:"cloud_name"`
	APIKey                types.String `tfsdk:"api_key"`
	APISecret             types.String `tfsdk:"api_secret"`
	AdminAPIBaseURL       types.String `tfsdk:"admin_api_base_url"`
}

// New returns a function that instantiates the provider with the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &cloudinaryProvider{version: version}
	}
}

func (p *cloudinaryProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "cloudinary"
	resp.Version = p.version
}

func (p *cloudinaryProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Cloudinary provider manages Cloudinary product environments (sub-accounts) and " +
			"their API access keys through the [Cloudinary Provisioning API](https://cloudinary.com/documentation/provisioning_api), " +
			"and upload presets and triggers through the [Admin API](https://cloudinary.com/documentation/admin_api). " +
			"Credentials are the account-level *provisioning* (account management) key and secret. The Admin API " +
			"authenticates per product environment, but `cloudinary_upload_preset` and `cloudinary_trigger` only " +
			"reference a `product_environment`; the provider resolves that environment's credentials itself.",
		Attributes: map[string]schema.Attribute{
			"account_id": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The Cloudinary account ID. May also be set with the `" + envAccountID +
					"` environment variable.",
			},
			"provisioning_api_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "The Cloudinary provisioning (account management) API key. May also be set with " +
					"the `" + envAPIKey + "` environment variable.",
			},
			"provisioning_api_secret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "The Cloudinary provisioning (account management) API secret. May also be set " +
					"with the `" + envAPISecret + "` environment variable.",
			},
			"api_region": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The regional Provisioning API endpoint to use: `api` (global, default), `api-eu` " +
					"or `api-ap`. May also be set with the `" + envRegion + "` environment variable.",
			},
			"api_base_url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Override the full Provisioning API base URL (e.g. for a proxy or testing). " +
					"Takes precedence over `api_region`. May also be set with the `" + envBaseURL + "` environment variable.",
			},
			"cloud_name": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Cloud name to use for Admin API resources instead of resolving one, for users " +
					"who hold product environment credentials but no provisioning credentials. May also be set with " +
					"the `" + envCloudName + "` environment variable.",
			},
			"api_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Default product environment API key for Admin API resources. May also be set " +
					"with the `" + envAdminAPIKey + "` environment variable.",
			},
			"api_secret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Default product environment API secret for Admin API resources. May also be set " +
					"with the `" + envAdminAPISecret + "` environment variable.",
			},
			"admin_api_base_url": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Override the Admin API base URL (e.g. for a proxy or testing). May also be set " +
					"with the `" + envAdminBaseURL + "` environment variable.",
			},
		},
	}
}

func (p *cloudinaryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config cloudinaryProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values take precedence over environment variables.
	accountID := firstNonEmpty(config.AccountID, os.Getenv(envAccountID))
	apiKey := firstNonEmpty(config.ProvisioningAPIKey, os.Getenv(envAPIKey))
	apiSecret := firstNonEmpty(config.ProvisioningAPISecret, os.Getenv(envAPISecret))
	region := firstNonEmpty(config.APIRegion, os.Getenv(envRegion))
	baseURL := firstNonEmpty(config.APIBaseURL, os.Getenv(envBaseURL))

	if accountID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("account_id"),
			"Missing Cloudinary Account ID",
			"Set the account_id attribute or the "+envAccountID+" environment variable.",
		)
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("provisioning_api_key"),
			"Missing Cloudinary Provisioning API Key",
			"Set the provisioning_api_key attribute or the "+envAPIKey+" environment variable.",
		)
	}
	if apiSecret == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("provisioning_api_secret"),
			"Missing Cloudinary Provisioning API Secret",
			"Set the provisioning_api_secret attribute or the "+envAPISecret+" environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	provisioning := newClient(clientConfig{
		AccountID: accountID,
		APIKey:    apiKey,
		APISecret: apiSecret,
		Region:    region,
		BaseURL:   baseURL,
	})

	clients := &providerClients{
		Provisioning: provisioning,
		Admin: newAdminResolver(provisioning, adminConfig{
			CloudName: firstNonEmpty(config.CloudName, os.Getenv(envCloudName)),
			APIKey:    firstNonEmpty(config.APIKey, os.Getenv(envAdminAPIKey)),
			APISecret: firstNonEmpty(config.APISecret, os.Getenv(envAdminAPISecret)),
			BaseURL:   firstNonEmpty(config.AdminAPIBaseURL, os.Getenv(envAdminBaseURL)),
		}),
	}

	// Make the clients available to resources and data sources.
	resp.ResourceData = clients
	resp.DataSourceData = clients
}

func (p *cloudinaryProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProductEnvironmentResource,
		NewAccessKeyResource,
		NewUploadPresetResource,
		NewTriggerResource,
	}
}

func (p *cloudinaryProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProductEnvironmentDataSource,
		NewAccessKeyDataSource,
		NewUploadPresetDataSource,
		NewTriggerDataSource,
	}
}

// firstNonEmpty returns the configured value if known and non-empty, otherwise
// the supplied fallback (typically an environment variable).
func firstNonEmpty(configured types.String, fallback string) string {
	if !configured.IsNull() && !configured.IsUnknown() && configured.ValueString() != "" {
		return configured.ValueString()
	}
	return fallback
}
