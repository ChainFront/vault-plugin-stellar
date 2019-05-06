/*
 * Copyright (c) 2019 ChainFront LLC.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stellar

import (
	"context"
	"fmt"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"github.com/shopspring/decimal"
	"github.com/stellar/go/keypair"
	"io/ioutil"
	"log"
	"net/http"
)

// Account is a Stellar account
type Account struct {
	Address      string   `json:"address"`
	Seed         string   `json:"seed"`
	AccountId    string   `json:"account_id"` // This is the original public key used to create the account
	TxSpendLimit string   `json:"tx_spend_limit"`
	Whitelist    []string `json:"whitelist"`
	Blacklist    []string `json:"blacklist"`
}

func accountsPaths(b *backend) []*framework.Path {
	return []*framework.Path{
		&framework.Path{
			Pattern: "accounts/?",
			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.ListOperation: b.pathListAccounts,
			},
		},
		&framework.Path{
			Pattern:      "accounts/" + framework.GenericNameRegex("name"),
			HelpSynopsis: "Create a Stellar account",
			Fields: map[string]*framework.FieldSchema{
				"name": &framework.FieldSchema{Type: framework.TypeString},
				"xlm_balance": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "(Optional) Initial starting balance of XLM",
				},
				"source_account_name": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "(Optional) Account used to fund the starting balance",
				},
				"tx_spend_limit": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "(Optional) Maximum amount of tokens which can be sent in a single transaction",
					Default:     "0",
				},
				"whitelist": &framework.FieldSchema{
					Type:        framework.TypeCommaStringSlice,
					Description: "(Optional) The list of accounts that this account can transact with.",
				},
				"blacklist": &framework.FieldSchema{
					Type:        framework.TypeCommaStringSlice,
					Description: "(Optional) The list of accounts that this account is forbidden from transacting with.",
				},
			},
			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.CreateOperation: b.pathCreateAccount,
				logical.UpdateOperation: b.pathCreateAccount,
				logical.ReadOperation:   b.pathReadAccount,
			},
		},
	}
}

// Returns a list of stored accounts (does not validate that the account is valid on Stellar)
func (b *backend) pathListAccounts(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	accountList, err := req.Storage.List(ctx, "accounts/")
	if err != nil {
		return nil, err
	}
	return logical.ListResponse(accountList), nil
}

// Using Stellar's SDK, generates and stores an ED25519 asymmetric key pair
func (b *backend) pathCreateAccount(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {

	// Validate we didn't get extra fields
	//err := validateFields(req, d)
	//if err != nil {
	//	return nil, logical.CodedError(422, err.Error())
	//}

	// Read optional fields
	var whitelist []string
	if whitelistRaw, ok := d.GetOk("whitelist"); ok {
		whitelist = whitelistRaw.([]string)
	}
	var blacklist []string
	if blacklistRaw, ok := d.GetOk("blacklist"); ok {
		blacklist = blacklistRaw.([]string)
	}

	txSpendLimitString := d.Get("tx_spend_limit").(string)
	txSpendLimit, err := decimal.NewFromString(txSpendLimitString)
	if err != nil || txSpendLimit.IsNegative() {
		return nil, fmt.Errorf("tx_spend_limit is either not a number or is negative")
	}

	// Generate a random KeyPair
	random, err := keypair.Random()
	if err != nil {
		log.Fatal(err)
	}

	// Get the public key and seed
	address := random.Address()
	seed := random.Seed()

	// Prod anchor
	//err = fundAccount(address)

	// Testnet
	err = fundTestAccount(address)
	if err != nil {
		log.Fatal(err)
	}

	// Create and store an Account object in Vault
	accountJSON := &Account{Address: address,
		Seed:         seed,
		AccountId:    address,
		TxSpendLimit: txSpendLimit.String(),
		Whitelist:    whitelist,
		Blacklist:    blacklist}

	entry, err := logical.StorageEntryJSON(req.Path, accountJSON)
	if err != nil {
		return nil, err
	}

	err = req.Storage.Put(ctx, entry)
	if err != nil {
		return nil, err
	}

	log.Printf("successfully created account %v", address)

	return &logical.Response{
		Data: map[string]interface{}{
			"address":          address,
			"stellarAccountId": address,
			"txSpendLimit":     txSpendLimit.String(),
			"whitelist":        whitelist,
			"blacklist":        blacklist,
		},
	}, nil
}

// Returns account details for the given account
func (b *backend) pathReadAccount(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {

	vaultAccount, err := b.readVaultAccount(ctx, req, req.Path)
	if err != nil {
		log.Fatal(err)
		return nil, fmt.Errorf("error reading account")
	}
	if vaultAccount == nil {
		return nil, nil
	}

	address := &vaultAccount.Address
	whitelist := &vaultAccount.Whitelist
	blacklist := &vaultAccount.Blacklist
	accountId := &vaultAccount.AccountId
	txSpendLimit := &vaultAccount.TxSpendLimit

	return &logical.Response{
		Data: map[string]interface{}{
			"address":          address,
			"stellarAccountId": accountId,
			"txSpendLimit":     txSpendLimit,
			"whitelist":        whitelist,
			"blacklist":        blacklist,
		},
	}, nil
}

func (b *backend) readVaultAccount(ctx context.Context, req *logical.Request, path string) (*Account, error) {
	log.Print("Reading account from path: " + path)
	entry, err := req.Storage.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read account at %s", path)
	}
	if entry == nil || len(entry.Value) == 0 {
		return nil, nil
	}

	var account Account
	err = entry.DecodeJSON(&account)

	if entry == nil {
		return nil, fmt.Errorf("failed to deserialize account at %s", path)
	}

	return &account, err
}

// Using the Stellar testnet Friendbot, fund a test account with some lumens
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
