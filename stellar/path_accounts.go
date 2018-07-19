package stellar

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/keypair"
)

type Account struct {
	Address string
	Seed    string
}

func accountsPaths(b *backend) []*framework.Path {
	return []*framework.Path{
		&framework.Path{
			Pattern: "accounts/?",
			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.ListOperation: b.listAccounts,
			},
		},
		&framework.Path{
			Pattern:      "accounts/" + framework.GenericNameRegex("name"),
			HelpSynopsis: "Create a Stellar account keypair",
			Fields: map[string]*framework.FieldSchema{
				"xlm_balance": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Initial balance of XLM",
					Default:     "1",
				},
			},
			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.CreateOperation: b.createAccount,
				logical.UpdateOperation: b.createAccount,
				logical.ReadOperation:   b.readAccount,
			},
		},
	}
}

// Returns a list of stored accounts (does not validate that the account is valid on Stellar)
func (b *backend) listAccounts(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	accountList, err := req.Storage.List(ctx, "accounts/")
	if err != nil {
		return nil, err
	}
	return logical.ListResponse(accountList), nil
}

// Using Stellar's SDK, generates and stores an ED25519 asymmetric key pair
func (b *backend) createAccount(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	random, err := keypair.Random()
	if err != nil {
		log.Fatal(err)
	}

	address := random.Address()
	seed := random.Seed()

	log.Print("Public key : " + address)

	// Prod anchor
	//err = fundAccount(address)

	// Testnet
	err = fundTestAccount(address)
	if err != nil {
		log.Fatal(err)
	}

	accountJSON := &Account{Address: address,
		Seed: seed}

	entry, err := logical.StorageEntryJSON(req.Path, accountJSON)
	if err != nil {
		return nil, err
	}

	err = req.Storage.Put(ctx, entry)
	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"address":      address,
			"created_time": time.Now(),
		},
	}, nil
}

func (b *backend) readAccount(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {

	vaultAccount, err := b.readVaultAccount(ctx, req, req.Path)
	address := &vaultAccount.Address

	stellarAccount, err := horizon.DefaultTestNetClient.LoadAccount(*address)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Balances for address:", address)
	for _, balance := range stellarAccount.Balances {
		log.Println(balance)
	}

	log.Print("Returning account...")
	return &logical.Response{
		Data: map[string]interface{}{
			"address":  address,
			"balances": stellarAccount.Balances,
		},
	}, nil
}

func (b *backend) readVaultAccount(ctx context.Context, req *logical.Request, path string) (*Account, error) {
	log.Print("Reading account from path: " + path)
	entry, err := req.Storage.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to find account at %s", path)
	}
	if entry == nil || len(entry.Value) == 0 {
		return nil, fmt.Errorf("no account found in storage")
	}

	log.Print("Deserializing account...")
	var account Account
	err = entry.DecodeJSON(&account)

	if entry == nil {
		return nil, fmt.Errorf("failed to deserialize account at %s", path)
	}

	return &account, err
}

func logBalances(account Account) {
	address := &account.Address

	stellarAccount, err := horizon.DefaultTestNetClient.LoadAccount(*address)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Balances for address:", address)
	for _, balance := range stellarAccount.Balances {
		log.Println(balance)
	}
	log.Print("Returning account...")
}

func fundTestAccount(address string) error {
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
