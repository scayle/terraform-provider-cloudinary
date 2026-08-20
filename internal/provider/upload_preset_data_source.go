package provider

import (
	"context"

	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*uploadPresetDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*uploadPresetDataSource)(nil)
)

type uploadPresetDataSource struct {
	resolver *adminResolver
}

func NewUploadPresetDataSource() datasource.DataSource {
	return &uploadPresetDataSource{}
}

func (d *uploadPresetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_upload_preset"
}

func (d *uploadPresetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Composite identifier in the form `<cloud_name>/<name>`.",
		},
		"name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The name of the upload preset to look up.",
		},
		"unsigned": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the preset allows unsigned uploads.",
		},
		"disallow_public_id": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether uploads using this preset are forbidden from specifying a public ID.",
		},
		"live": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the preset may be used for live streaming uploads.",
		},
	}

	for name, attr := range adminReferenceDataSourceAttributes() {
		attrs[name] = attr
	}
	for name, attr := range uploadPresetDataSourceAttributes() {
		attrs[name] = attr
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Cloudinary upload preset from a product environment.",
		Attributes:          attrs,
	}
}

func (d *uploadPresetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	clients := configureClients(req.ProviderData, &resp.Diagnostics)
	if clients == nil {
		return
	}
	d.resolver = clients.Admin
}

func (d *uploadPresetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	environment, accessKey := adminReference(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	client, creds := d.resolver.clientFor(ctx, environment, accessKey, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := client.GetUploadPreset(ctx, admin.GetUploadPresetParams{Name: name.ValueString()})
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading Cloudinary upload preset", err.Error())
		return
	}

	resp.State.Raw = req.Config.Raw

	settings := settingsAsMap(res.Settings)
	presetSettingsToState(ctx, settings, &resp.State, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(res.Name))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("unsigned"), types.BoolValue(res.Unsigned))...)
	setBoolFromSettings(ctx, settings, "disallow_public_id", &resp.State, &resp.Diagnostics)
	setBoolFromSettings(ctx, settings, "live", &resp.State, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), uploadPresetID(creds.CloudName, res.Name))...)
}
