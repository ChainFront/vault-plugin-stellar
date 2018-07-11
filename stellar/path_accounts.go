package stellar

import (
	"context"

	"github.com/stellar/go/keypair"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Account struct {
	Address string
	Seed    string
}

func accountsPaths(b *backend) []*framework.Path {
	return []*framework.Path{
		&framework.Path{
			Pattern:      "accounts/" + framework.GenericNameRegex("name"),
			HelpSynopsis: "Create a Stellar account keypair",
			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.CreateOperation: b.pathKeypair,
			},
		},
	}
}

// Using Vault's transit backend, generates and stores an ED25519 asymmetric key pair
func (b *backend) pathKeypair(_ context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {

	random, err := keypair.Random()
	if err != nil {
		log.Fatal(err)
	}

	address := random.Address()
	seed := random.Seed()

	log.Print("Public key : " + address)

	err = FundTestAccount(address)
	if err != nil {
		log.Fatal(err)
	}

	accountJSON := &Account{Address: address,
		Seed: seed}

	logical.StorageEntryJSON(req.Path, accountJSON)

	return &logical.Response{
		Data: map[string]interface{}{
			"address":      address,
			"created_time": time.Now(),
		},
	}, nil
}

func FundTestAccount(address string) error {
	resp, err := http.Get("https://horizon-testnet.stellar.org/friendbot?addr=" + address)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if _, err := ioutil.ReadAll(resp.Body); err != nil {
		return err
	}

	return nil
}
