package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type presetFieldKind int

const (
	presetString presetFieldKind = iota
	presetBool
	presetFloat
	presetStringList
	presetStringMap
)

// presetField maps a Terraform attribute onto the identically named parameter
// of the Admin API's upload preset settings.
type presetField struct {
	name      string
	kind      presetFieldKind
	sensitive bool
	desc      string
}

// uploadPresetFields covers the upload parameters that can be stored in a
// preset. Image-only structured parameters (access_control, face_coordinates,
// custom_coordinates, regions, responsive_breakpoints) are not represented.
var uploadPresetFields = []presetField{
	{name: "public_id", kind: presetString, desc: "Identifier used for accessing and delivering the uploaded asset."},
	{name: "public_id_prefix", kind: presetString, desc: "String prepended to the public ID with a slash."},
	{name: "filename_override", kind: presetString, desc: "Sets the `original-filename` metadata header instead of the uploaded filename."},
	{name: "display_name", kind: presetString, desc: "User-friendly name for the asset. Defaults to the public ID."},
	{name: "folder", kind: presetString, desc: "Folder to place the asset in. Legacy fixed folder mode only; use `asset_folder` in dynamic folder mode."},
	{name: "asset_folder", kind: presetString, desc: "Full path of the folder the asset is placed in. Dynamic folder mode only."},
	{name: "resource_type", kind: presetString, desc: "File type: `image`, `raw`, `video` or `auto`."},
	{name: "type", kind: presetString, desc: "Delivery type: `upload`, `private` or `authenticated`. Defaults to `upload`."},
	{name: "transformation", kind: presetString, desc: "Incoming transformation to apply before saving."},
	{name: "format", kind: presetString, desc: "Format to convert the asset to before saving, e.g. `mp4`."},
	{name: "eager", kind: presetString, desc: "Pipe-separated list of eager transformations to create on upload."},
	{name: "moderation", kind: presetString, desc: "Moderation behaviour, depending on the enabled add-ons."},
	{name: "raw_convert", kind: presetString, desc: "Generate a related file based on the uploaded raw file."},
	{name: "categorization", kind: presetString, desc: "Comma-separated list of categorization add-ons."},
	{name: "background_removal", kind: presetString, desc: "Background removal add-on to apply."},
	{name: "detection", kind: presetString, desc: "Detection add-on to apply."},
	{name: "ocr", kind: presetString, desc: "OCR add-on to apply."},
	{name: "headers", kind: presetString, desc: "HTTP headers to add to responses when delivering the asset."},
	{name: "notification_url", kind: presetString, desc: "Webhook URL to receive the upload response."},
	{name: "eager_notification_url", kind: presetString, desc: "Webhook URL to notify when eager transformations complete."},
	{name: "on_success", kind: presetString, desc: "JavaScript logic to update the asset after a successful upload."},
	{
		name:      "eval",
		kind:      presetString,
		sensitive: true,
		desc: "JavaScript logic evaluated before the upload finalises, used to modify upload parameters. It is " +
			"treated as sensitive because it is also used to carry credentials into a preset.",
	},

	{name: "use_filename", kind: presetBool, desc: "Use the original filename as the public ID when none is given."},
	{name: "unique_filename", kind: presetBool, desc: "Append random characters to the filename to ensure uniqueness."},
	{name: "use_filename_as_display_name", kind: presetBool, desc: "Assign the original filename as the display name."},
	{name: "unique_display_name", kind: presetBool, desc: "Ensure the display name is unique within its asset folder."},
	{name: "use_asset_folder_as_public_id_prefix", kind: presetBool, desc: "Prefix the public ID with the asset folder."},
	{name: "overwrite", kind: presetBool, desc: "Overwrite an existing asset with the same public ID."},
	{name: "discard_original_filename", kind: presetBool, desc: "Discard the original filename."},
	{name: "backup", kind: presetBool, desc: "Back up the uploaded asset, overriding the product environment default."},
	{name: "invalidate", kind: presetBool, desc: "Invalidate CDN cached copies of the asset and its transformations."},
	{name: "eager_async", kind: presetBool, desc: "Create eager transformations asynchronously."},
	{name: "faces", kind: presetBool, desc: "Return the coordinates of detected faces."},
	{name: "image_metadata", kind: presetBool, desc: "Return the image metadata."},
	{name: "media_metadata", kind: presetBool, desc: "Return IPTC, XMP and detailed Exif metadata."},
	{name: "exif", kind: presetBool, desc: "Return the Exif metadata."},
	{name: "colors", kind: presetBool, desc: "Return the predominant colors of the asset."},
	{name: "phash", kind: presetBool, desc: "Return the perceptual hash of the asset."},
	{name: "quality_analysis", kind: presetBool, desc: "Return a quality analysis score."},
	{name: "accessibility_analysis", kind: presetBool, desc: "Return an accessibility analysis."},
	{name: "cinemagraph_analysis", kind: presetBool, desc: "Return a cinemagraph score. Animated images and video only."},
	{name: "auto_chaptering", kind: presetBool, desc: "Auto-generate video chapters. Video only."},
	{name: "visual_search", kind: presetBool, desc: "Index the asset for visual search."},

	{name: "auto_tagging", kind: presetFloat, desc: "Confidence threshold for automatic tagging, between 0.0 and 1.0."},

	{name: "tags", kind: presetStringList, desc: "Tags to assign to the asset."},
	{name: "allowed_formats", kind: presetStringList, desc: "Formats accepted for upload. Any supported type is allowed when unset."},

	{name: "context", kind: presetStringMap, desc: "Key-value contextual metadata to attach to the asset."},
	{name: "metadata", kind: presetStringMap, desc: "Custom metadata fields, keyed by external ID."},
}

func uploadPresetSchemaAttributes() map[string]schema.Attribute {
	attrs := map[string]schema.Attribute{}
	for _, f := range uploadPresetFields {
		switch f.kind {
		case presetString:
			attrs[f.name] = schema.StringAttribute{Optional: true, Sensitive: f.sensitive, MarkdownDescription: f.desc}
		case presetBool:
			attrs[f.name] = schema.BoolAttribute{Optional: true, MarkdownDescription: f.desc}
		case presetFloat:
			attrs[f.name] = schema.Float64Attribute{Optional: true, MarkdownDescription: f.desc}
		case presetStringList:
			attrs[f.name] = schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: f.desc}
		case presetStringMap:
			attrs[f.name] = schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: f.desc}
		}
	}
	return attrs
}

func uploadPresetDataSourceAttributes() map[string]dsschema.Attribute {
	attrs := map[string]dsschema.Attribute{}
	for _, f := range uploadPresetFields {
		switch f.kind {
		case presetString:
			attrs[f.name] = dsschema.StringAttribute{Computed: true, Sensitive: f.sensitive, MarkdownDescription: f.desc}
		case presetBool:
			attrs[f.name] = dsschema.BoolAttribute{Computed: true, MarkdownDescription: f.desc}
		case presetFloat:
			attrs[f.name] = dsschema.Float64Attribute{Computed: true, MarkdownDescription: f.desc}
		case presetStringList:
			attrs[f.name] = dsschema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: f.desc}
		case presetStringMap:
			attrs[f.name] = dsschema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: f.desc}
		}
	}
	return attrs
}

// attributeGetter is implemented by tfsdk.Plan, tfsdk.State and tfsdk.Config.
type attributeGetter interface {
	GetAttribute(ctx context.Context, p path.Path, target any) diag.Diagnostics
}

// attributeSetter is implemented by tfsdk.State.
type attributeSetter interface {
	SetAttribute(ctx context.Context, p path.Path, val any) diag.Diagnostics
}

// presetSettingsFromConfig collects the configured upload parameters into a map
// keyed by their Admin API names. Null attributes are omitted so Cloudinary
// keeps its own defaults.
func presetSettingsFromConfig(ctx context.Context, src attributeGetter, diags *diag.Diagnostics) map[string]any {
	settings := map[string]any{}

	for _, f := range uploadPresetFields {
		p := path.Root(f.name)

		switch f.kind {
		case presetString:
			var v types.String
			diags.Append(src.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				settings[f.name] = v.ValueString()
			}
		case presetBool:
			var v types.Bool
			diags.Append(src.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				settings[f.name] = v.ValueBool()
			}
		case presetFloat:
			var v types.Float64
			diags.Append(src.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				settings[f.name] = v.ValueFloat64()
			}
		case presetStringList:
			var v types.List
			diags.Append(src.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				var items []string
				diags.Append(v.ElementsAs(ctx, &items, false)...)
				settings[f.name] = items
			}
		case presetStringMap:
			var v types.Map
			diags.Append(src.GetAttribute(ctx, p, &v)...)
			if !v.IsNull() && !v.IsUnknown() {
				items := map[string]string{}
				diags.Append(v.ElementsAs(ctx, &items, false)...)
				settings[f.name] = items
			}
		}
	}

	return settings
}

// presetSettingsToState writes the settings the Admin API returned back into
// state. Parameters the API does not report are cleared, so a preset edited
// outside Terraform shows up as drift.
func presetSettingsToState(ctx context.Context, settings map[string]any, dst attributeSetter, diags *diag.Diagnostics) {
	for _, f := range uploadPresetFields {
		p := path.Root(f.name)
		raw, present := settings[f.name]

		switch f.kind {
		case presetString:
			value := basetypes.NewStringNull()
			if s, ok := raw.(string); present && ok {
				value = types.StringValue(s)
			}
			diags.Append(dst.SetAttribute(ctx, p, value)...)
		case presetBool:
			value := basetypes.NewBoolNull()
			if b, ok := raw.(bool); present && ok {
				value = types.BoolValue(b)
			}
			diags.Append(dst.SetAttribute(ctx, p, value)...)
		case presetFloat:
			value := basetypes.NewFloat64Null()
			if n, ok := raw.(float64); present && ok {
				value = types.Float64Value(n)
			}
			diags.Append(dst.SetAttribute(ctx, p, value)...)
		case presetStringList:
			value := types.ListNull(types.StringType)
			if items, ok := toStringSlice(raw); present && ok {
				list, listDiags := types.ListValueFrom(ctx, types.StringType, items)
				diags.Append(listDiags...)
				value = list
			}
			diags.Append(dst.SetAttribute(ctx, p, value)...)
		case presetStringMap:
			value := types.MapNull(types.StringType)
			if items, ok := toStringMap(raw); present && ok {
				m, mapDiags := types.MapValueFrom(ctx, types.StringType, items)
				diags.Append(mapDiags...)
				value = m
			}
			diags.Append(dst.SetAttribute(ctx, p, value)...)
		}
	}
}

// settingsAsMap normalises the untyped settings object the Admin API returns on
// reads. Create and update take a typed struct, but reads come back as free-form
// JSON, so it has to be re-decoded here.
func settingsAsMap(raw any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	if m, ok := raw.(map[string]any); ok {
		return m
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return map[string]any{}
	}
	m := map[string]any{}
	if err := json.Unmarshal(encoded, &m); err != nil {
		return map[string]any{}
	}
	return m
}

// toStringSlice also accepts the comma-joined form, which is how the SDK
// serialises list parameters on the way out and how Cloudinary sometimes
// echoes them back.
func toStringSlice(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		return v, true
	case string:
		if v == "" {
			return []string{}, true
		}
		return strings.Split(v, ","), true
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			items = append(items, s)
		}
		return items, true
	default:
		return nil, false
	}
}

// toStringMap also accepts the pipe-separated "key=value" form the SDK sends.
func toStringMap(raw any) (map[string]string, bool) {
	switch v := raw.(type) {
	case map[string]string:
		return v, true
	case string:
		if v == "" {
			return map[string]string{}, true
		}
		items := map[string]string{}
		for _, pair := range strings.Split(v, "|") {
			key, value, ok := strings.Cut(pair, "=")
			if !ok {
				return nil, false
			}
			items[key] = value
		}
		return items, true
	case map[string]any:
		items := make(map[string]string, len(v))
		for key, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			items[key] = s
		}
		return items, true
	default:
		return nil, false
	}
}

// warnUnstoredSettings reports parameters that were configured but that
// Cloudinary did not store. It silently discards parameters whose add-on is not
// enabled for the product environment, which would otherwise show up only as a
// diff that never converges.
func warnUnstoredSettings(configured, stored map[string]any, diags *diag.Diagnostics) {
	var missing []string
	for _, f := range uploadPresetFields {
		if _, ok := configured[f.name]; !ok {
			continue
		}
		if _, ok := stored[f.name]; !ok {
			missing = append(missing, f.name)
		}
	}
	if len(missing) == 0 {
		return
	}

	sort.Strings(missing)
	diags.AddWarning(
		"Cloudinary did not store some configured parameters",
		fmt.Sprintf("The upload preset was written, but Cloudinary discarded: %s. This usually means the add-on "+
			"a parameter depends on is not enabled for this product environment. The next plan will show a diff "+
			"for these parameters until they are removed from the configuration or the add-on is enabled.",
			strings.Join(missing, ", ")),
	)
}
