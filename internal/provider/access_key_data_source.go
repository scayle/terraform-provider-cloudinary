package provider

import (
	"context"
	"fmt"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/operations"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*accessKeyDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*accessKeyDataSource)(nil)
)

type accessKeyDataSource struct {
	client *cldprovisioning.CldProvisioning
}

type accessKeyDataSourceModel struct {
	SubAccountID types.String `tfsdk:"sub_account_id"`
	APIKey       types.String `tfsdk:"api_key"`
	Name         types.String `tfsdk:"name"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func NewAccessKeyDataSource() datasource.DataSource {
	return &accessKeyDataSource{}
}

func (d *accessKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_key"
}

func (d *accessKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Cloudinary API access key by sub-account ID and API key. The API secret is " +
			"never returned by the read API and is therefore not exposed by this data source.",
		Attributes: map[string]schema.Attribute{
			"sub_account_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the product environment (sub-account) the key belongs to.",
			},
			"api_key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The API key to look up.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The name of the access key.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the access key is enabled.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The RFC 3339 timestamp when the access key was created.",
			},
		},
	}
}

func (d *accessKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *accessKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config accessKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := d.client.AccessKeys.List(ctx, operations.GetAccessKeysRequest{
		SubAccountID: config.SubAccountID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading Cloudinary access key", err.Error())
		return
	}

	wanted := config.APIKey.ValueString()
	for i := range res.AccessKeys {
		k := res.AccessKeys[i]
		if deref(k.APIKey) != wanted {
			continue
		}
		config.Name = types.StringValue(deref(k.Name))
		config.Enabled = types.BoolPointerValue(k.Enabled)
		config.CreatedAt = timeToStringValue(k.CreatedAt)
		resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
		return
	}

	resp.Diagnostics.AddError(
		"Cloudinary access key not found",
		fmt.Sprintf("No access key with api_key %q was found in sub-account %q.", wanted, config.SubAccountID.ValueString()),
	)
}
