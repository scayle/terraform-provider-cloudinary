package provider

import (
	"context"
	"time"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/components"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/operations"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// apiAccessKeyAttrTypes describes an access-key object (key/secret/enabled).
var apiAccessKeyAttrTypes = map[string]attr.Type{
	"key":     types.StringType,
	"secret":  types.StringType,
	"enabled": types.BoolType,
}

var (
	_ resource.Resource                = (*productEnvironmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*productEnvironmentResource)(nil)
	_ resource.ResourceWithImportState = (*productEnvironmentResource)(nil)
)

type productEnvironmentResource struct {
	client *cldprovisioning.CldProvisioning
}

type productEnvironmentResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	CloudName        types.String `tfsdk:"cloud_name"`
	BaseSubAccountID types.String `tfsdk:"base_sub_account_id"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	CreatedAt        types.String `tfsdk:"created_at"`
	InitialAccessKey types.Object `tfsdk:"initial_access_key"`
}

func NewProductEnvironmentResource() resource.Resource {
	return &productEnvironmentResource{}
}

func (r *productEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_product_environment"
}

func (r *productEnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cloudinary product environment (previously known as a sub-account).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the product environment (sub-account).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The display name of the product environment.",
			},
			"cloud_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The Cloudinary cloud name. If omitted, Cloudinary auto-generates one.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"base_sub_account_id": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The ID of an existing product environment to copy settings from. Only used at " +
					"creation time; changing it forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the product environment is enabled. Defaults to `true`.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The RFC 3339 timestamp when the product environment was created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"initial_access_key": schema.SingleNestedAttribute{
				Computed: true,
				MarkdownDescription: "The API access key automatically provisioned together with the sub-account. " +
					"Only populated when Terraform creates the sub-account; it is `null` after an import because the API " +
					"does not identify which of a sub-account's keys was the original. The `secret` is only returned at " +
					"creation time. To manage or read other access keys, use the `cloudinary_access_key` resource/data source.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"key": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The API key.",
					},
					"secret": schema.StringAttribute{
						Computed:            true,
						Sensitive:           true,
						MarkdownDescription: "The API secret. Only populated at creation time.",
					},
					"enabled": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "Whether the access key is enabled.",
					},
				},
			},
		},
	}
}

func (r *productEnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *productEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan productEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := components.ProductEnvironmentRequest{Name: plan.Name.ValueString()}
	if !plan.CloudName.IsNull() && !plan.CloudName.IsUnknown() {
		createReq.CloudName = cldprovisioning.String(plan.CloudName.ValueString())
	}
	if !plan.BaseSubAccountID.IsNull() {
		createReq.BaseSubAccountID = cldprovisioning.String(plan.BaseSubAccountID.ValueString())
	}

	env, err := r.client.ProductEnvironments.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Cloudinary sub-account", err.Error())
		return
	}

	// The create endpoint always provisions an enabled environment. If the user
	// requested it disabled, apply that with an update.
	if !plan.Enabled.ValueBool() {
		env, err = r.client.ProductEnvironments.Update(ctx, operations.UpdateProductEnvironmentRequest{
			SubAccountID:                    deref(env.ID),
			ProductEnvironmentUpdateRequest: components.ProductEnvironmentUpdateRequest{Enabled: cldprovisioning.Bool(false)},
		})
		if err != nil {
			resp.Diagnostics.AddError("Error disabling Cloudinary sub-account after creation", err.Error())
			return
		}
	}

	// Preserve the freshly returned access-key secrets in state.
	resp.Diagnostics.Append(r.mapEnvToModel(ctx, env, &plan, nil)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *productEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state productEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.ProductEnvironments.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Cloudinary sub-account", err.Error())
		return
	}

	prior := state // capture secrets known from previous applies
	resp.Diagnostics.Append(r.mapEnvToModel(ctx, env, &state, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *productEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state productEnvironmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := components.ProductEnvironmentUpdateRequest{
		Name:    cldprovisioning.String(plan.Name.ValueString()),
		Enabled: cldprovisioning.Bool(plan.Enabled.ValueBool()),
	}
	if !plan.CloudName.IsNull() && !plan.CloudName.IsUnknown() {
		updateReq.CloudName = cldprovisioning.String(plan.CloudName.ValueString())
	}

	env, err := r.client.ProductEnvironments.Update(ctx, operations.UpdateProductEnvironmentRequest{
		SubAccountID:                    state.ID.ValueString(),
		ProductEnvironmentUpdateRequest: updateReq,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Cloudinary sub-account", err.Error())
		return
	}

	prior := state
	resp.Diagnostics.Append(r.mapEnvToModel(ctx, env, &plan, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *productEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state productEnvironmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.ProductEnvironments.Delete(ctx, state.ID.ValueString()); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting Cloudinary sub-account", err.Error())
	}
}

func (r *productEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// The import ID may be a sub-account ID or a cloud name; resolve to the ID
	// so the subsequent Read (which looks up by ID) succeeds.
	id, err := resolveProductEnvironmentID(ctx, r.client, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Cloudinary sub-account", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// mapEnvToModel writes the API response onto model.
//
// initial_access_key holds only the key auto-provisioned with the sub-account:
//   - On create (prior == nil), the create response returns exactly that key,
//     including its secret.
//   - On read/update, the create-time key is identified by matching the api_key
//     already stored in prior state (the read API returns every key but omits
//     secrets, so the stored secret is retained).
//   - After an import there is no stored key to match, so it stays null.
func (r *productEnvironmentResource) mapEnvToModel(_ context.Context, env *components.ProductEnvironment, model *productEnvironmentResourceModel, prior *productEnvironmentResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(deref(env.ID))
	model.Name = types.StringValue(deref(env.Name))
	model.CloudName = types.StringValue(deref(env.CloudName))
	if env.Enabled != nil {
		model.Enabled = types.BoolValue(*env.Enabled)
	}
	model.CreatedAt = timeToStringValue(env.CreatedAt)

	var chosen *components.APIAccessKey
	var priorSecret string
	if prior == nil {
		// Create: the response carries exactly the auto-provisioned key.
		if len(env.APIAccessKeys) > 0 {
			chosen = &env.APIAccessKeys[0]
		}
	} else if priorKey, secret := initialKeyFromState(prior.InitialAccessKey); priorKey != "" {
		priorSecret = secret
		for i := range env.APIAccessKeys {
			if deref(env.APIAccessKeys[i].Key) == priorKey {
				chosen = &env.APIAccessKeys[i]
				break
			}
		}
	}

	if chosen == nil {
		model.InitialAccessKey = types.ObjectNull(apiAccessKeyAttrTypes)
		return diags
	}

	secret := deref(chosen.Secret)
	if secret == "" {
		secret = priorSecret
	}
	obj, d := types.ObjectValue(apiAccessKeyAttrTypes, map[string]attr.Value{
		"key":     types.StringValue(deref(chosen.Key)),
		"secret":  nullableString(secret),
		"enabled": types.BoolPointerValue(chosen.Enabled),
	})
	diags.Append(d...)
	model.InitialAccessKey = obj

	return diags
}

// initialKeyFromState extracts the api_key and secret previously stored in the
// initial_access_key object, so the create-time key can be re-identified and its
// secret retained across refreshes.
func initialKeyFromState(obj types.Object) (key, secret string) {
	if obj.IsNull() || obj.IsUnknown() {
		return "", ""
	}
	attrs := obj.Attributes()
	if v, ok := attrs["key"].(types.String); ok {
		key = v.ValueString()
	}
	if v, ok := attrs["secret"].(types.String); ok {
		secret = v.ValueString()
	}
	return key, secret
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullableString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func timeToStringValue(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}
