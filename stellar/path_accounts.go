package stellar

import (
	"context"

	"github.com/stellar/go/keypair"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"log"
)

func (b *backend) pathKeypair(_ context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	random, err := keypair.Random()
	if err != nil {
		log.Fatal(err)
	}

	address := random.Address()
	random.Seed()

	return &logical.Response{
		Data: map[string]interface{}{
			"address": address,
		},
	}, nil
}
