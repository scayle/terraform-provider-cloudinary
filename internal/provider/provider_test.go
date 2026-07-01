package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestProviderSchema builds the provider server and fetches the full schema.
// GetProviderSchema validates the provider, every resource, and every data
// source schema, so this catches invalid attribute combinations at unit-test
// time without needing Terraform or live credentials.
func TestProviderSchema(t *testing.T) {
	t.Parallel()

	server := providerserver.NewProtocol6(New("test")())()

	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema returned error: %s", err)
	}

	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema diagnostic error: %s: %s", d.Summary, d.Detail)
		}
	}

	wantResources := []string{"cloudinary_sub_account", "cloudinary_access_key"}
	for _, name := range wantResources {
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("missing resource schema %q", name)
		}
	}

	wantDataSources := []string{"cloudinary_sub_account", "cloudinary_access_key"}
	for _, name := range wantDataSources {
		if _, ok := resp.DataSourceSchemas[name]; !ok {
			t.Errorf("missing data source schema %q", name)
		}
	}
}
