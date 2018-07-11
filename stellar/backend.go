package stellar

import (
	"context"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"github.com/pkg/errors"
)

type backend struct {
	*framework.Backend
}

// Factory creates a new usable instance of this secrets engine.
func Factory(ctx context.Context, c *logical.BackendConfig) (logical.Backend, error) {
	b := Backend(c)
	if err := b.Setup(ctx, c); err != nil {
		return nil, errors.Wrap(err, "Unable to set up Stellar secret backend")
	}
	return b, nil
}

func Backend(c *logical.BackendConfig) *backend {
	var b backend
	b.Backend = &framework.Backend{
		Help: "",
		Paths: framework.PathAppend(
			accountsPaths(&b),
		),
		PathsSpecial: &logical.Paths{},
		Secrets:      []*framework.Secret{},
		BackendType:  logical.TypeLogical,
	}
	return &b
}
