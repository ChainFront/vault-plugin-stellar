package stellar

import (
	"context"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"github.com/stellar/go/build"
	"log"
)

func paymentsPaths(b *backend) []*framework.Path {
	return []*framework.Path{
		&framework.Path{
			Pattern:      "payments",
			HelpSynopsis: "Make a payment on the Stellar network",
			Fields: map[string]*framework.FieldSchema{
				"sequenceNum": &framework.FieldSchema{
					Type:        framework.TypeInt,
					Default:     0,
					Description: "Sequence number for this transaction",
				},
				"source": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Source account",
				},
				"destination": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Destination account",
				},
				"amount": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Amount to send",
				},
			},
			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.CreateOperation: b.createPayment,
				logical.UpdateOperation: b.createPayment,
			},
		},
	}
}

// Using Vault's transit backend, generates and stores an ED25519 asymmetric key pair
func (b *backend) createPayment(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {

	// Validate we didn't get extra fields
	err := validateFields(req, d)
	if err != nil {
		return nil, logical.CodedError(422, err.Error())
	}

	// Validate required fields are present
	sequenceNum := d.Get("sequenceNum").(int)
	if sequenceNum == 0 {
		return errMissingField("sequenceNum"), nil
	}
	source := d.Get("source").(string)
	if source == "" {
		return errMissingField("source"), nil
	}
	destination := d.Get("destination").(string)
	if destination == "" {
		return errMissingField("destination"), nil
	}
	amount := d.Get("amount").(string)
	if amount == "" {
		return errMissingField("amount"), nil
	}

	// Retrieve the source account keypair from vault storage
	sourceAccount, err := b.readVaultAccount(ctx, req, "accounts/"+source)
	if err != nil {
		log.Fatal(err)
	}
	sourceAddress := sourceAccount.Address

	// Retrieve the destination account keypair from vault storage
	destinationAccount, err := b.readVaultAccount(ctx, req, "accounts/"+destination)
	if err != nil {
		log.Fatal(err)
	}
	destinationAddress := destinationAccount.Address

	// Build the transaction
	// TODO: multisig
	// TODO: custom assets
	tx, err := build.Transaction(
		build.SourceAccount{AddressOrSeed: sourceAddress},
		build.TestNetwork,
		build.Sequence{uint64(sequenceNum)},
		build.Payment(
			build.Destination{AddressOrSeed: destinationAddress},
			build.NativeAmount{Amount: amount},
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Sign the transaction
	signedTx, err := tx.Sign(sourceAccount.Seed)
	if err != nil {
		log.Fatal(err)
	}

	// Convert to base64
	signedTxBase64, err := signedTx.Base64()
	if err != nil {
		log.Fatal(err)
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"signedTx": signedTxBase64,
		},
	}, nil
}
