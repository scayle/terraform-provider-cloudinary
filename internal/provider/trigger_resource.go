package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*triggerResource)(nil)
	_ resource.ResourceWithConfigure   = (*triggerResource)(nil)
	_ resource.ResourceWithImportState = (*triggerResource)(nil)
)

type triggerResource struct {
	resolver *adminResolver
}

type triggerResourceModel struct {
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
	AuthScheme         types.String `tfsdk:"auth_scheme"`
	URIType            types.String `tfsdk:"uri_type"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func NewTriggerResource() resource.Resource {
	return &triggerResource{}
}

func (r *triggerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trigger"
}

func (r *triggerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Composite identifier in the form `<cloud_name>/<trigger_id>`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"trigger_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID Cloudinary assigned to the trigger.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"uri": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The endpoint notifications are delivered to.",
		},
		"event_type": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The event that fires the trigger, for example `upload`.",
		},
		"additive": schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Whether the trigger is delivered in addition to any globally configured notification URL. Defaults to `false`.",
		},
		"filter": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "JSON-encoded filter restricting which events fire the trigger.",
		},
		"filter_language": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "The language the `filter` expression is written in.",
		},
		"payload_template": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "JSON-encoded template shaping the notification payload.",
		},
		"auth_scheme": schema.StringAttribute{
			Optional:            true,
			Sensitive:           true,
			MarkdownDescription: "Authentication scheme presented to the endpoint. Treated as sensitive because it may carry credentials.",
		},
		"uri_type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The transport Cloudinary inferred from `uri`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "When the trigger was created.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"updated_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "When the trigger was last updated.",
		},
	}

	for name, attr := range adminReferenceAttributes() {
		attrs[name] = attr
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cloudinary trigger within a product environment through the " +
			"[Admin API](https://cloudinary.com/documentation/admin_api#create_a_trigger). Triggers deliver webhook " +
			"notifications, for example when an asset finishes uploading.",
		Attributes: attrs,
	}
}

func (r *triggerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	clients := configureClients(req.ProviderData, &resp.Diagnostics)
	if clients == nil {
		return
	}
	r.resolver = clients.Admin
}

func (r *triggerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan triggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, creds := r.clientFor(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	params := admin.CreateTriggerParams{
		URI:            plan.URI.ValueString(),
		EventType:      plan.EventType.ValueString(),
		Additive:       plan.Additive.ValueBool(),
		FilterLanguage: plan.FilterLanguage.ValueString(),
		AuthScheme:     plan.AuthScheme.ValueString(),
	}
	params.Filter = decodeJSONObject(plan.Filter, "filter", &resp.Diagnostics)
	params.PayloadTemplate = decodeJSONObject(plan.PayloadTemplate, "payload_template", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := client.CreateTrigger(ctx, params)
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error creating Cloudinary trigger", err.Error())
		return
	}

	mapTriggerToModel(&res.Trigger, &plan, creds.CloudName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *triggerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state triggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, creds := r.clientFor(ctx, state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	trigger, err := lookupTrigger(ctx, client, state.TriggerID.ValueString(), "")
	if err != nil {
		if isAdminNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Cloudinary trigger", err.Error())
		return
	}
	if trigger == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	mapTriggerToModel(trigger, &state, creds.CloudName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *triggerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state triggerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, creds := r.clientFor(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	params := admin.UpdateTriggerParams{
		TriggerID:      state.TriggerID.ValueString(),
		URI:            plan.URI.ValueString(),
		EventType:      plan.EventType.ValueString(),
		FilterLanguage: plan.FilterLanguage.ValueString(),
		AuthScheme:     plan.AuthScheme.ValueString(),
	}
	if !plan.Additive.IsNull() && !plan.Additive.IsUnknown() {
		additive := plan.Additive.ValueBool()
		params.Additive = &additive
	}
	params.Filter = decodeJSONObject(plan.Filter, "filter", &resp.Diagnostics)
	params.PayloadTemplate = decodeJSONObject(plan.PayloadTemplate, "payload_template", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := client.UpdateTrigger(ctx, params)
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating Cloudinary trigger", err.Error())
		return
	}

	// The update response does not carry the trigger, so mapping it straight
	// back would blank every computed attribute. Re-read instead, and fall back
	// to the response only if the trigger cannot be found.
	trigger := &res.Trigger
	if refreshed, err := lookupTrigger(ctx, client, state.TriggerID.ValueString(), ""); err == nil && refreshed != nil {
		trigger = refreshed
	} else if trigger.ID == "" {
		trigger.ID = state.TriggerID.ValueString()
	}

	mapTriggerToModel(trigger, &plan, creds.CloudName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *triggerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state triggerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client, _ := r.clientFor(ctx, state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := client.DeleteTrigger(ctx, admin.DeleteTriggerParams{TriggerID: state.TriggerID.ValueString()})
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil && !isAdminNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Cloudinary trigger", err.Error())
	}
}

func (r *triggerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	environment, triggerID, ok := strings.Cut(req.ID, "/")
	if !ok || environment == "" || triggerID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import identifier in the form \"<product_environment>/<trigger_id>\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("product_environment"), environment)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("trigger_id"), triggerID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), environment+"/"+triggerID)...)
}

func (r *triggerResource) clientFor(ctx context.Context, model triggerResourceModel, diags *diag.Diagnostics) (*admin.API, adminConfig) {
	return r.resolver.clientFor(ctx, model.ProductEnvironment.ValueString(), model.AccessKey.ValueString(), diags)
}

func mapTriggerToModel(trigger *admin.Trigger, model *triggerResourceModel, cloudName string) {
	model.TriggerID = types.StringValue(trigger.ID)
	model.ID = types.StringValue(cloudName + "/" + trigger.ID)
	model.URI = types.StringValue(trigger.URI)
	model.EventType = types.StringValue(trigger.EventType)
	model.Additive = types.BoolValue(trigger.Additive)
	model.URIType = nullableString(trigger.URIType)
	model.CreatedAt = nullableString(trigger.CreatedAt)
	model.UpdatedAt = nullableString(trigger.UpdatedAt)
	model.FilterLanguage = nullableString(trigger.FilterLanguage)
	model.Filter = encodeJSONObject(trigger.Filter)
	model.PayloadTemplate = encodeJSONObject(trigger.PayloadTemplate)
}

func decodeJSONObject(value types.String, attribute string, diags *diag.Diagnostics) map[string]any {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return nil
	}
	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(value.ValueString()), &decoded); err != nil {
		diags.AddAttributeError(
			path.Root(attribute),
			"Invalid JSON",
			fmt.Sprintf("Expected %q to contain a JSON object: %s", attribute, err),
		)
		return nil
	}
	return decoded
}

func encodeJSONObject(value map[string]any) types.String {
	if len(value) == 0 {
		return types.StringNull()
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return types.StringNull()
	}
	return types.StringValue(string(encoded))
}
