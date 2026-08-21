package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*triggerDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*triggerDataSource)(nil)
)

type triggerDataSource struct {
	resolver *adminResolver
}

func NewTriggerDataSource() datasource.DataSource {
	return &triggerDataSource{}
}

func (d *triggerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trigger"
}

func (d *triggerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Composite identifier in the form `<cloud_name>/<trigger_id>`.",
		},
		"trigger_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "The ID of the trigger to look up. Either this or `uri` must be set.",
		},
		"uri": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "The endpoint notifications are delivered to. May be used to look the trigger up instead of `trigger_id`.",
		},
		"event_type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The event that fires the trigger.",
		},
		"additive": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether the trigger is delivered in addition to any globally configured notification URL.",
		},
		"filter": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "JSON-encoded filter restricting which events fire the trigger.",
		},
		"filter_language": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The language the `filter` expression is written in.",
		},
		"payload_template": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "JSON-encoded template shaping the notification payload.",
		},
		"uri_type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The transport Cloudinary inferred from `uri`.",
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "When the trigger was created.",
		},
		"updated_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "When the trigger was last updated.",
		},
	}

	for name, attr := range adminReferenceDataSourceAttributes() {
		attrs[name] = attr
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a Cloudinary trigger from a product environment.",
		Attributes:          attrs,
	}
}

func (d *triggerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	clients := configureClients(req.ProviderData, &resp.Diagnostics)
	if clients == nil {
		return
	}
	d.resolver = clients.Admin
}

type triggerDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	ProductEnvironment types.String `tfsdk:"product_environment"`
	AccessKey          types.String `tfsdk:"access_key"`
	TriggerID          types.String `tfsdk:"trigger_id"`
	URI                types.String `tfsdk:"uri"`
	EventType          types.String `tfsdk:"event_type"`
	Additive           types.Bool   `tfsdk:"additive"`
	Filter             types.String `tfsdk:"filter"`
	FilterLanguage     types.String `tfsdk:"filter_language"`
	PayloadTemplate    types.String `tfsdk:"payload_template"`
	URIType            types.String `tfsdk:"uri_type"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func (d *triggerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config triggerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.TriggerID.ValueString() == "" && config.URI.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing Trigger Identifier",
			"Set either trigger_id or uri to identify the trigger to read.",
		)
		return
	}

	client, creds := d.resolver.clientFor(ctx, config.ProductEnvironment.ValueString(), config.AccessKey.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	trigger, err := lookupTrigger(ctx, client, config.TriggerID.ValueString(), config.URI.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Cloudinary trigger", err.Error())
		return
	}
	if trigger == nil {
		resp.Diagnostics.AddError(
			"Cloudinary trigger not found",
			fmt.Sprintf("No trigger matching id %q or uri %q was found.", config.TriggerID.ValueString(), config.URI.ValueString()),
		)
		return
	}

	config.TriggerID = types.StringValue(trigger.ID)
	config.ID = types.StringValue(creds.CloudName + "/" + trigger.ID)
	config.URI = types.StringValue(trigger.URI)
	config.EventType = types.StringValue(trigger.EventType)
	config.Additive = types.BoolValue(trigger.Additive)
	config.URIType = nullableString(trigger.URIType)
	config.CreatedAt = nullableString(trigger.CreatedAt)
	config.UpdatedAt = nullableString(trigger.UpdatedAt)
	config.FilterLanguage = nullableString(trigger.FilterLanguage)
	config.Filter = encodeJSONObject(trigger.Filter)
	config.PayloadTemplate = encodeJSONObject(trigger.PayloadTemplate)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
