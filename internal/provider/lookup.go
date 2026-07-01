package provider

import (
	"context"
	"fmt"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/components"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/operations"
)

// getSubAccountByCloudName returns the product environment whose cloud name
// matches, or (nil, nil) if none exists.
func getSubAccountByCloudName(ctx context.Context, client *cldprovisioning.CldProvisioning, cloudName string) (*components.ProductEnvironment, error) {
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

// resolveSubAccountID resolves a reference that may be either a sub-account ID
// or a cloud name into the sub-account ID.
func resolveSubAccountID(ctx context.Context, client *cldprovisioning.CldProvisioning, ref string) (string, error) {
	// Try the reference as an ID first. Any failure (the API returns varying
	// statuses/messages for an unknown or malformed ID) simply means we fall
	// back to treating the reference as a cloud name.
	if env, err := client.ProductEnvironments.Get(ctx, ref); err == nil {
		return deref(env.ID), nil
	}

	env, err := getSubAccountByCloudName(ctx, client, ref)
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
