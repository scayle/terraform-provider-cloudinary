package provider

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/cloudinary/account-provisioning-go/cldprovisioning"
	"github.com/cloudinary/account-provisioning-go/cldprovisioning/models/sdkerrors"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// clientConfig holds the resolved provider credentials and endpoint settings.
type clientConfig struct {
	AccountID string
	APIKey    string
	APISecret string
	Region    string
	BaseURL   string
}

// envMu serialises the transient environment mutation in newClient. The
// Cloudinary SDK only reads provisioning credentials from environment variables
// at construction time (its WithSecurity/WithSecuritySource options are affected
// by a value-vs-pointer method-set bug), so we set them just long enough for
// cldprovisioning.New to copy them into its account hook.
var envMu sync.Mutex

// newClient builds a Cloudinary provisioning client from the resolved config.
func newClient(cfg clientConfig) *cldprovisioning.CldProvisioning {
	opts := []cldprovisioning.SDKOption{cldprovisioning.WithAccountID(cfg.AccountID)}
	if cfg.Region != "" {
		opts = append(opts, cldprovisioning.WithRegion(cldprovisioning.ServerRegion(cfg.Region)))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, cldprovisioning.WithServerURL(cfg.BaseURL))
	}

	envMu.Lock()
	defer envMu.Unlock()

	restore := setEnv(map[string]string{
		"CLOUDINARY_ACCOUNT_ID":              cfg.AccountID,
		"CLOUDINARY_PROVISIONING_API_KEY":    cfg.APIKey,
		"CLOUDINARY_PROVISIONING_API_SECRET": cfg.APISecret,
	})
	defer restore()

	return cldprovisioning.New(opts...)
}

// setEnv sets the given environment variables and returns a function that
// restores their previous values.
func setEnv(vars map[string]string) func() {
	previous := make(map[string]*string, len(vars))
	for k, v := range vars {
		if old, ok := os.LookupEnv(k); ok {
			prev := old
			previous[k] = &prev
		} else {
			previous[k] = nil
		}
		_ = os.Setenv(k, v)
	}
	return func() {
		for k, old := range previous {
			if old == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *old)
			}
		}
	}
}

// providerClients carries the Provisioning API client plus the provider-level
// Admin API credential defaults, which Admin API resources may override.
type providerClients struct {
	Provisioning *cldprovisioning.CldProvisioning
	Admin        adminConfig
}

func configureClients(providerData any, diags *diag.Diagnostics) *providerClients {
	if providerData == nil {
		return nil
	}

	clients, ok := providerData.(*providerClients)
	if !ok {
		diags.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *providerClients, got: %T. Please report this issue to the provider developers.", providerData),
		)
		return nil
	}

	return clients
}

// configureClient extracts the *cldprovisioning.CldProvisioning client from the
// provider-supplied data. providerData is nil during validation passes, which is
// not an error. A wrong type is a programming error and is reported via diags.
func configureClient(providerData any, diags *diag.Diagnostics) *cldprovisioning.CldProvisioning {
	clients := configureClients(providerData, diags)
	if clients == nil {
		return nil
	}
	return clients.Provisioning
}

// isNotFound reports whether err is a Cloudinary API error with a 404 status,
// allowing callers to remove a resource from state when it no longer exists.
func isNotFound(err error) bool {
	var apiErr *sdkerrors.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}
