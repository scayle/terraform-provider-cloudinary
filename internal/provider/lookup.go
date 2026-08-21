package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/components"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/operations"
	"github.com/cloudinary/cloudinary-go/v2/api/admin"
)

// getProductEnvironmentByCloudName returns the product environment whose cloud name
// matches, or (nil, nil) if none exists.
func getProductEnvironmentByCloudName(ctx context.Context, client *cldprovisioning.CldProvisioning, cloudName string) (*components.ProductEnvironment, error) {
	res, err := client.ProductEnvironments.List(ctx, &operations.GetProductEnvironmentsRequest{
		CloudNames: []string{cloudName},
	})
	if err != nil {
		return nil, err
	}
	for i := range res.SubAccounts {
		if deref(res.SubAccounts[i].CloudName) == cloudName {
			return &res.SubAccounts[i], nil
		}
	}
	return nil, nil
}

// resolveProductEnvironmentID resolves a reference that may be either a sub-account ID
// or a cloud name into the sub-account ID.
func resolveProductEnvironmentID(ctx context.Context, client *cldprovisioning.CldProvisioning, ref string) (string, error) {
	// Try the reference as an ID first. Any failure (the API returns varying
	// statuses/messages for an unknown or malformed ID) simply means we fall
	// back to treating the reference as a cloud name.
	if env, err := client.ProductEnvironments.Get(ctx, ref); err == nil {
		return deref(env.ID), nil
	}

	env, err := getProductEnvironmentByCloudName(ctx, client, ref)
	if err != nil {
		return "", err
	}
	if env == nil {
		return "", fmt.Errorf("no sub-account found with id or cloud_name %q", ref)
	}
	return deref(env.ID), nil
}

// resolveAccessKey finds an access key within a sub-account by matching keyRef
// against its api_key first, then its name. Returns (nil, nil) if not found.
func resolveAccessKey(ctx context.Context, client *cldprovisioning.CldProvisioning, subAccountID, keyRef string) (*components.AccessKey, error) {
	res, err := client.AccessKeys.List(ctx, operations.GetAccessKeysRequest{SubAccountID: subAccountID})
	if err != nil {
		return nil, err
	}
	for i := range res.AccessKeys {
		if deref(res.AccessKeys[i].APIKey) == keyRef {
			return &res.AccessKeys[i], nil
		}
	}
	for i := range res.AccessKeys {
		if deref(res.AccessKeys[i].Name) == keyRef {
			return &res.AccessKeys[i], nil
		}
	}
	return nil, nil
}

func lookupTrigger(ctx context.Context, client *admin.API, triggerID, uri string) (*admin.Trigger, error) {
	res, err := client.ListTriggers(ctx, admin.ListTriggersParams{})
	if err == nil && res != nil {
		err = adminError(res.Error)
	}
	if err != nil {
		return nil, err
	}
	for i := range res.Triggers {
		t := &res.Triggers[i]
		if triggerID != "" && t.ID == triggerID {
			return t, nil
		}
		if triggerID == "" && uri != "" && t.URI == uri {
			return t, nil
		}
	}
	return nil, nil
}

func resolveProductEnvironment(ctx context.Context, client *cldprovisioning.CldProvisioning, ref string) (*components.ProductEnvironment, error) {
	if env, err := client.ProductEnvironments.Get(ctx, ref); err == nil {
		return env, nil
	}

	env, err := getProductEnvironmentByCloudName(ctx, client, ref)
	if err != nil {
		return nil, err
	}
	if env == nil {
		return nil, fmt.Errorf("no sub-account found with id or cloud_name %q", ref)
	}
	return env, nil
}

// rootAccessKeyName is what Cloudinary calls the access key it provisions
// alongside a product environment.
const rootAccessKeyName = "Root"

// pickAccessKey chooses the key the Admin API calls authenticate with: the one
// named in the configuration, else the root key, else the oldest enabled key.
//
// The Provisioning API marks the root key with a "root" flag, but the generated
// SDK model omits it, so the key is identified by name. The oldest-enabled
// fallback covers an environment whose root key was renamed or removed.
func pickAccessKey(keys []components.AccessKey, name string) *components.AccessKey {
	if name == "" {
		name = rootAccessKeyName
	}

	for i := range keys {
		if deref(keys[i].Name) == name {
			return &keys[i]
		}
	}
	if name != rootAccessKeyName {
		return nil
	}

	var chosen *components.AccessKey
	for i := range keys {
		k := &keys[i]
		if k.Enabled != nil && !*k.Enabled {
			continue
		}
		if chosen == nil || olderThan(k.CreatedAt, chosen.CreatedAt) {
			chosen = k
		}
	}
	return chosen
}

func olderThan(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return a.Before(*b)
}
