package provider

import (
	"context"
	"fmt"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/components"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*productEnvironmentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*productEnvironmentDataSource)(nil)
)

type productEnvironmentDataSource struct {
	client *cldprovisioning.CldProvisioning
}

type productEnvironmentDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	CloudName     types.String `tfsdk:"cloud_name"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	CreatedAt     types.String `tfsdk:"created_at"`
	APIAccessKeys types.List   `tfsdk:"api_access_keys"`
}

func NewProductEnvironmentDataSource() datasource.DataSource {
	return &productEnvironmentDataSource{}
}

func (d *productEnvironmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_product_environment"
}

func (d *productEnvironmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Cloudinary product environment (sub-account) by its `id` or by its `cloud_name`. " +
			"Exactly one of the two must be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The ID of the product environment (sub-account) to look up. Conflicts with `cloud_name`.",
			},
			"cloud_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The Cloudinary cloud name of the product environment to look up. Conflicts with `id`.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The display name of the product environment.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the product environment is enabled.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The RFC 3339 timestamp when the product environment was created.",
			},
			"api_access_keys": schema.ListNestedAttribute{
				Computed: true,
				MarkdownDescription: "The API access keys of the sub-account. The `secret` is not returned by the read " +
					"API and is therefore empty here.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The API key.",
						},
						"secret": schema.StringAttribute{
							Computed:            true,
							Sensitive:           true,
							MarkdownDescription: "The API secret. Not returned by the read API.",
						},
						"enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the access key is enabled.",
						},
					},
				},
			},
		},
	}
}

func (d *productEnvironmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *productEnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config productEnvironmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	byID := !config.ID.IsNull() && config.ID.ValueString() != ""
	byCloudName := !config.CloudName.IsNull() && config.CloudName.ValueString() != ""

	if byID == byCloudName {
		resp.Diagnostics.AddError(
			"Invalid Cloudinary sub-account lookup",
			"Exactly one of \"id\" or \"cloud_name\" must be set.",
		)
		return
	}

	var env *components.ProductEnvironment
	var err error
	if byID {
		env, err = d.client.ProductEnvironments.Get(ctx, config.ID.ValueString())
		if isNotFound(err) {
			env, err = nil, nil
		}
	} else {
		env, err = getProductEnvironmentByCloudName(ctx, d.client, config.CloudName.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Cloudinary sub-account", err.Error())
		return
	}
	if env == nil {
		detail := fmt.Sprintf("No sub-account with cloud_name %q exists on this account.", config.CloudName.ValueString())
		if byID {
			detail = fmt.Sprintf("No sub-account with id %q exists on this account.", config.ID.ValueString())
		}
		resp.Diagnostics.AddError("Cloudinary sub-account not found", detail)
		return
	}

	config.ID = types.StringValue(deref(env.ID))
	config.Name = types.StringValue(deref(env.Name))
	config.CloudName = types.StringValue(deref(env.CloudName))
	config.Enabled = types.BoolPointerValue(env.Enabled)
	config.CreatedAt = timeToStringValue(env.CreatedAt)

	keys := make([]attr.Value, 0, len(env.APIAccessKeys))
	for _, k := range env.APIAccessKeys {
		obj, diags := types.ObjectValue(apiAccessKeyAttrTypes, map[string]attr.Value{
			"key":     types.StringValue(deref(k.Key)),
			"secret":  nullableString(deref(k.Secret)),
			"enabled": types.BoolPointerValue(k.Enabled),
		})
		resp.Diagnostics.Append(diags...)
		keys = append(keys, obj)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: apiAccessKeyAttrTypes}, keys)
	resp.Diagnostics.Append(diags...)
	config.APIAccessKeys = list

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
