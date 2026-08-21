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
	_ resource.Resource                = (*uploadPresetResource)(nil)
	_ resource.ResourceWithConfigure   = (*uploadPresetResource)(nil)
	_ resource.ResourceWithImportState = (*uploadPresetResource)(nil)
)

type uploadPresetResource struct {
	resolver *adminResolver
}

func NewUploadPresetResource() resource.Resource {
	return &uploadPresetResource{}
}

func (r *uploadPresetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_upload_preset"
}

func (r *uploadPresetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Composite identifier in the form `<cloud_name>/<name>`.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The name of the upload preset. Changing it forces a new resource.",
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"unsigned": schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			Default:             booldefault.StaticBool(false),
			MarkdownDescription: "Whether the preset allows unsigned uploads. Defaults to `false`.",
		},
		"disallow_public_id": schema.BoolAttribute{
			Optional:            true,
			MarkdownDescription: "Whether uploads using this preset are forbidden from specifying a public ID.",
		},
		"live": schema.BoolAttribute{
			Optional:            true,
			MarkdownDescription: "Whether the preset may be used for live streaming uploads.",
		},
	}

	for name, attr := range adminReferenceAttributes() {
		attrs[name] = attr
	}
	for name, attr := range uploadPresetSchemaAttributes() {
		attrs[name] = attr
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cloudinary upload preset within a product environment through the " +
			"[Admin API](https://cloudinary.com/documentation/admin_api#upload_presets). Upload presets bundle the " +
			"parameters applied to every upload that references them.",
		Attributes: attrs,
	}
}

func (r *uploadPresetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	clients := configureClients(req.ProviderData, &resp.Diagnostics)
	if clients == nil {
		return
	}
	r.resolver = clients.Admin
}

func (r *uploadPresetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	client, creds := r.clientFor(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var name types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("name"), &name)...)

	settings := presetSettingsFromConfig(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var params admin.CreateUploadPresetParams
	if err := decodeSettings(settings, &params); err != nil {
		resp.Diagnostics.AddError("Error building Cloudinary upload preset request", err.Error())
		return
	}
	params.Name = name.ValueString()
	params.Unsigned = optionalBool(ctx, req.Plan, "unsigned", &resp.Diagnostics)
	params.DisallowPublicID = optionalBool(ctx, req.Plan, "disallow_public_id", &resp.Diagnostics)
	params.Live = optionalBool(ctx, req.Plan, "live", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := client.CreateUploadPreset(ctx, params)
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error creating Cloudinary upload preset", err.Error())
		return
	}

	r.warnUnstored(ctx, client, name.ValueString(), settings, &resp.Diagnostics)

	resp.State.Raw = req.Plan.Raw
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), uploadPresetID(creds.CloudName, name.ValueString()))...)
}

func (r *uploadPresetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	client, creds := r.clientFor(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var name types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := client.GetUploadPreset(ctx, admin.GetUploadPresetParams{Name: name.ValueString()})
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil {
		if isAdminNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Cloudinary upload preset", err.Error())
		return
	}

	settings := settingsAsMap(res.Settings)
	presetSettingsToState(ctx, settings, &resp.State, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(res.Name))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("unsigned"), types.BoolValue(res.Unsigned))...)
	setBoolFromSettings(ctx, settings, "disallow_public_id", &resp.State, &resp.Diagnostics)
	setBoolFromSettings(ctx, settings, "live", &resp.State, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), uploadPresetID(creds.CloudName, res.Name))...)
}

func (r *uploadPresetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	client, creds := r.clientFor(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var name types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("name"), &name)...)

	settings := presetSettingsFromConfig(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var params admin.UpdateUploadPresetParams
	if err := decodeSettings(settings, &params); err != nil {
		resp.Diagnostics.AddError("Error building Cloudinary upload preset request", err.Error())
		return
	}
	params.Name = name.ValueString()
	params.Unsigned = optionalBool(ctx, req.Plan, "unsigned", &resp.Diagnostics)
	params.DisallowPublicID = optionalBool(ctx, req.Plan, "disallow_public_id", &resp.Diagnostics)
	params.Live = optionalBool(ctx, req.Plan, "live", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := client.UpdateUploadPreset(ctx, params)
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating Cloudinary upload preset", err.Error())
		return
	}

	r.warnUnstored(ctx, client, name.ValueString(), settings, &resp.Diagnostics)

	resp.State.Raw = req.Plan.Raw
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), uploadPresetID(creds.CloudName, name.ValueString()))...)
}

func (r *uploadPresetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	client, _ := r.clientFor(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var name types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := client.DeleteUploadPreset(ctx, admin.DeleteUploadPresetParams{Name: name.ValueString()})
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil && !isAdminNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Cloudinary upload preset", err.Error())
	}
}

func (r *uploadPresetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	environment, name, ok := strings.Cut(req.ID, "/")
	if !ok || environment == "" || name == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import identifier in the form \"<product_environment>/<name>\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("product_environment"), environment)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), uploadPresetID(environment, name))...)
}

// clientFor resolves the Admin API credentials from the given plan or state,
// falling back to the provider-level defaults.
func (r *uploadPresetResource) clientFor(ctx context.Context, src attributeGetter, diags *diag.Diagnostics) (*admin.API, adminConfig) {
	environment, accessKey := adminReference(ctx, src, diags)
	if diags.HasError() {
		return nil, adminConfig{}
	}
	return r.resolver.clientFor(ctx, environment, accessKey, diags)
}

// warnUnstored re-reads the preset so a parameter Cloudinary silently ignored
// is reported at apply time rather than only as a recurring diff.
func (r *uploadPresetResource) warnUnstored(ctx context.Context, client *admin.API, name string, configured map[string]any, diags *diag.Diagnostics) {
	res, err := client.GetUploadPreset(ctx, admin.GetUploadPresetParams{Name: name})
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil {
		return
	}
	warnUnstoredSettings(configured, settingsAsMap(res.Settings), diags)
}

func uploadPresetID(cloudName, name string) string {
	return cloudName + "/" + name
}

// decodeSettings moves the collected parameters into the SDK's typed request
// struct via JSON, which is also how the SDK serialises them on the wire.
func decodeSettings(settings map[string]any, target any) error {
	encoded, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

func optionalBool(ctx context.Context, src attributeGetter, name string, diags *diag.Diagnostics) *bool {
	var v types.Bool
	diags.Append(src.GetAttribute(ctx, path.Root(name), &v)...)
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	value := v.ValueBool()
	return &value
}

func setBoolFromSettings(ctx context.Context, settings map[string]any, name string, dst attributeSetter, diags *diag.Diagnostics) {
	value := types.BoolNull()
	if b, ok := settings[name].(bool); ok {
		value = types.BoolValue(b)
	}
	diags.Append(dst.SetAttribute(ctx, path.Root(name), value)...)
}
