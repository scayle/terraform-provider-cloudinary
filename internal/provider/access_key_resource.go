package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/components"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/operations"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*accessKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*accessKeyResource)(nil)
	_ resource.ResourceWithImportState = (*accessKeyResource)(nil)
)

type accessKeyResource struct {
	client *cldprovisioning.CldProvisioning
}

type accessKeyResourceModel struct {
	ID           types.String `tfsdk:"id"`
	SubAccountID types.String `tfsdk:"sub_account_id"`
	Name         types.String `tfsdk:"name"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	APIKey       types.String `tfsdk:"api_key"`
	APISecret    types.String `tfsdk:"api_secret"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func NewAccessKeyResource() resource.Resource {
	return &accessKeyResource{}
}

func (r *accessKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_key"
}

func (r *accessKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cloudinary API access key within a product environment (sub-account). " +
			"Access keys are used to generate signed upload parameters for a tenant space.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Composite identifier in the form `<sub_account_id>/<api_key>`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"sub_account_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the product environment (sub-account) the key belongs to. Changing it forces a new resource.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The name of the access key. If omitted, Cloudinary assigns one.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the access key is enabled. Defaults to `true`.",
			},
			"api_key": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The generated API key.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"api_secret": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The generated API secret. Only returned at creation time; unavailable after import.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The RFC 3339 timestamp when the access key was created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *accessKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *accessKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accessKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := components.AccessKeyRequest{Enabled: cldprovisioning.Bool(plan.Enabled.ValueBool())}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body.Name = cldprovisioning.String(plan.Name.ValueString())
	}

	key, err := r.client.AccessKeys.Generate(ctx, operations.GenerateAccessKeyRequest{
		SubAccountID:     plan.SubAccountID.ValueString(),
		AccessKeyRequest: body,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Cloudinary access key", err.Error())
		return
	}

	mapAccessKeyToModel(key, &plan, "")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accessKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accessKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.findAccessKey(ctx, state.SubAccountID.ValueString(), state.APIKey.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Cloudinary access key", err.Error())
		return
	}
	if key == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// The list endpoint does not return the secret; keep the one already in state.
	mapAccessKeyToModel(key, &state, state.APISecret.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accessKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state accessKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.client.AccessKeys.Update(ctx, operations.UpdateAccessKeyRequest{
		SubAccountID: state.SubAccountID.ValueString(),
		Key:          state.APIKey.ValueString(),
		AccessKeyUpdateRequest: components.AccessKeyUpdateRequest{
			Name:    cldprovisioning.String(plan.Name.ValueString()),
			Enabled: cldprovisioning.Bool(plan.Enabled.ValueBool()),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating Cloudinary access key", err.Error())
		return
	}

	// Preserve the secret captured at creation time.
	mapAccessKeyToModel(key, &plan, state.APISecret.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accessKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accessKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.AccessKeys.Delete(ctx, operations.DeleteAccessKeyRequest{
		SubAccountID: state.SubAccountID.ValueString(),
		Key:          state.APIKey.ValueString(),
	})
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Cloudinary access key", err.Error())
	}
}

func (r *accessKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	subRef, keyRef, ok := splitAccessKeyID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import identifier in the form \"<sub_account_id>/<api_key>\" or \"<cloud_name>/<key_name>\", got: %q", req.ID),
		)
		return
	}

	// The sub-account reference may be an ID or a cloud name.
	subAccountID, err := resolveSubAccountID(ctx, r.client, subRef)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Cloudinary access key", err.Error())
		return
	}

	// The key reference may be an api_key or a key name.
	key, err := resolveAccessKey(ctx, r.client, subAccountID, keyRef)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Cloudinary access key", err.Error())
		return
	}
	if key == nil {
		resp.Diagnostics.AddError(
			"Cloudinary access key not found",
			fmt.Sprintf("No access key with api_key or name %q was found in sub-account %q.", keyRef, subAccountID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("sub_account_id"), subAccountID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("api_key"), deref(key.APIKey))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), subAccountID+"/"+deref(key.APIKey))...)
}

// findAccessKey lists the access keys of a sub-account and returns the one
// matching apiKey, or nil if absent.
func (r *accessKeyResource) findAccessKey(ctx context.Context, subAccountID, apiKey string) (*components.AccessKey, error) {
	res, err := r.client.AccessKeys.List(ctx, operations.GetAccessKeysRequest{SubAccountID: subAccountID})
	if err != nil {
		return nil, err
	}
	for i := range res.AccessKeys {
		if deref(res.AccessKeys[i].APIKey) == apiKey {
			return &res.AccessKeys[i], nil
		}
	}
	return nil, nil
}

// mapAccessKeyToModel writes the API response onto model. secretOverride is used
// to retain a previously known secret when the API response omits it.
func mapAccessKeyToModel(key *components.AccessKey, model *accessKeyResourceModel, secretOverride string) {
	// model.SubAccountID is already populated from plan/state by the caller.
	model.APIKey = types.StringValue(deref(key.APIKey))
	model.Name = types.StringValue(deref(key.Name))
	if key.Enabled != nil {
		model.Enabled = types.BoolValue(*key.Enabled)
	}
	model.CreatedAt = timeToStringValue(key.CreatedAt)

	secret := deref(key.APISecret)
	if secret == "" {
		secret = secretOverride
	}
	model.APISecret = nullableString(secret)
	model.ID = types.StringValue(model.SubAccountID.ValueString() + "/" + model.APIKey.ValueString())
}

func splitAccessKeyID(id string) (subAccountID, apiKey string, ok bool) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
