package provider

import (
	"context"
	"testing"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/components"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/operations"
)

// TestIntegrationSDKAgainstMock exercises the exact SDK calls the resources make
// against the in-memory mock, verifying request/response wiring end to end
// without requiring Terraform or live credentials.
func TestIntegrationSDKAgainstMock(t *testing.T) {
	t.Parallel()

	mock := newMockProvisioning()
	ts := mock.server()
	defer ts.Close()

	// Build the client through the provider's own helper, exercising the
	// env-based credential injection the SDK actually honours.
	client := newClient(clientConfig{
		AccountID: "acct",
		APIKey:    "k",
		APISecret: "s",
		BaseURL:   ts.URL,
	})
	ctx := context.Background()

	// --- Product environment create ---
	env, err := client.ProductEnvironments.Create(ctx, components.ProductEnvironmentRequest{
		Name:      "acme",
		CloudName: cldprovisioning.String("acme-prod"),
	})
	if err != nil {
		t.Fatalf("create env: %s", err)
	}
	if deref(env.CloudName) != "acme-prod" {
		t.Errorf("cloud_name = %q, want acme-prod", deref(env.CloudName))
	}
	if len(env.APIAccessKeys) != 1 || deref(env.APIAccessKeys[0].Secret) == "" {
		t.Fatalf("expected one api access key with a secret at creation, got %+v", env.APIAccessKeys)
	}
	envID := deref(env.ID)

	// --- Sub-account read ---
	got, err := client.ProductEnvironments.Get(ctx, envID)
	if err != nil {
		t.Fatalf("get env: %s", err)
	}
	if deref(got.Name) != "acme" {
		t.Errorf("name = %q, want acme", deref(got.Name))
	}
	// On read the boot secret is omitted, exercising the preserve-secret path.
	if deref(got.APIAccessKeys[0].Secret) != "" {
		t.Errorf("read should not return the boot secret")
	}

	// --- Sub-account update ---
	updated, err := client.ProductEnvironments.Update(ctx, operations.UpdateProductEnvironmentRequest{
		SubAccountID: envID,
		ProductEnvironmentUpdateRequest: components.ProductEnvironmentUpdateRequest{
			Name:    cldprovisioning.String("acme-renamed"),
			Enabled: cldprovisioning.Bool(false),
		},
	})
	if err != nil {
		t.Fatalf("update env: %s", err)
	}
	if deref(updated.Name) != "acme-renamed" || updated.Enabled == nil || *updated.Enabled {
		t.Errorf("update not applied: %+v", updated)
	}

	// --- Access key generate ---
	key, err := client.AccessKeys.Generate(ctx, operations.GenerateAccessKeyRequest{
		SubAccountID:     envID,
		AccessKeyRequest: components.AccessKeyRequest{Name: cldprovisioning.String("live")},
	})
	if err != nil {
		t.Fatalf("generate key: %s", err)
	}
	if deref(key.APISecret) == "" {
		t.Fatalf("expected a secret at key generation")
	}
	apiKey := deref(key.APIKey)

	// --- Access key list/find (the resource Read path) ---
	keys, err := client.AccessKeys.List(ctx, operations.GetAccessKeysRequest{SubAccountID: envID})
	if err != nil {
		t.Fatalf("list keys: %s", err)
	}
	var found *components.AccessKey
	for i := range keys.AccessKeys {
		if deref(keys.AccessKeys[i].APIKey) == apiKey {
			found = &keys.AccessKeys[i]
		}
	}
	if found == nil {
		t.Fatalf("generated key %q not found in list", apiKey)
	}
	if deref(found.APISecret) == "" {
		t.Errorf("list should return the secret")
	}

	// --- Access key delete then sub-account delete ---
	if _, err := client.AccessKeys.Delete(ctx, operations.DeleteAccessKeyRequest{SubAccountID: envID, Key: apiKey}); err != nil {
		t.Fatalf("delete key: %s", err)
	}
	if _, err := client.ProductEnvironments.Delete(ctx, envID); err != nil {
		t.Fatalf("delete env: %s", err)
	}
	if _, err := client.ProductEnvironments.Get(ctx, envID); !isNotFound(err) {
		t.Errorf("expected not-found after delete, got %v", err)
	}
}
