package provider

import (
	"net/http"
	"testing"
	"time"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/components"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/sdkerrors"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSplitAccessKeyID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		wantSub string
		wantKey string
		wantOK  bool
	}{
		{"sub123/key456", "sub123", "key456", true},
		{"sub123", "", "", false},
		{"/key456", "", "", false},
		{"sub123/", "", "", false},
		{"", "", "", false},
		{"sub/key/extra", "sub", "key/extra", true}, // SplitN keeps the remainder
	}

	for _, c := range cases {
		sub, key, ok := splitAccessKeyID(c.in)
		if sub != c.wantSub || key != c.wantKey || ok != c.wantOK {
			t.Errorf("splitAccessKeyID(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, sub, key, ok, c.wantSub, c.wantKey, c.wantOK)
		}
	}
}

func TestNullableString(t *testing.T) {
	t.Parallel()

	if got := nullableString(""); !got.IsNull() {
		t.Errorf("nullableString(\"\") = %v, want null", got)
	}
	if got := nullableString("x"); got.ValueString() != "x" {
		t.Errorf("nullableString(\"x\") = %v, want \"x\"", got)
	}
}

func TestTimeToStringValue(t *testing.T) {
	t.Parallel()

	if got := timeToStringValue(nil); !got.IsNull() {
		t.Errorf("timeToStringValue(nil) = %v, want null", got)
	}
	ts := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	if got := timeToStringValue(&ts); got.ValueString() != "2026-06-30T12:00:00Z" {
		t.Errorf("timeToStringValue = %v, want RFC3339", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	t.Parallel()

	if got := firstNonEmpty(types.StringValue("cfg"), "env"); got != "cfg" {
		t.Errorf("config value should win, got %q", got)
	}
	if got := firstNonEmpty(types.StringNull(), "env"); got != "env" {
		t.Errorf("null config should fall back to env, got %q", got)
	}
	if got := firstNonEmpty(types.StringValue(""), "env"); got != "env" {
		t.Errorf("empty config should fall back to env, got %q", got)
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	if !isNotFound(sdkerrors.NewAPIError("nope", http.StatusNotFound, "", nil)) {
		t.Error("404 APIError should be treated as not found")
	}
	if isNotFound(sdkerrors.NewAPIError("boom", http.StatusInternalServerError, "", nil)) {
		t.Error("500 APIError should not be treated as not found")
	}
	if isNotFound(http.ErrServerClosed) {
		t.Error("non-API error should not be treated as not found")
	}
}

func TestMapAccessKeyToModelPreservesSecret(t *testing.T) {
	t.Parallel()

	enabled := true
	apiKey := "123"
	name := "main_key"
	key := &components.AccessKey{APIKey: &apiKey, Name: &name, Enabled: &enabled}

	model := &accessKeyResourceModel{SubAccountID: types.StringValue("sub1")}
	// List/Update responses omit the secret; the previously stored value must survive.
	mapAccessKeyToModel(key, model, "kept-secret")

	if model.APISecret.ValueString() != "kept-secret" {
		t.Errorf("secret not preserved, got %q", model.APISecret.ValueString())
	}
	if model.ID.ValueString() != "sub1/123" {
		t.Errorf("composite id = %q, want sub1/123", model.ID.ValueString())
	}
}
