package stellar

import (
	"context"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"log"
)

// Register the callbacks for the paths exposed by these functions
func paymentsPaths(b *backend) []*framework.Path {
	return []*framework.Path{
		&framework.Path{
			Pattern:      "payments",
			HelpSynopsis: "Make a payment on the Stellar network",
			Fields: map[string]*framework.FieldSchema{
				"source": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Source account",
				},
				"destination": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Destination account",
				},
				"paymentChannel": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Payment channel account",
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

// Creates a signed transaction with a payment operation.
func (b *backend) createPayment(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {

	// Validate we didn't get extra fields
	err := validateFields(req, d)
	if err != nil {
		return nil, logical.CodedError(422, err.Error())
	}

	// Validate required fields are present
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

	// Read optional fields
	paymentChannel := d.Get("paymentChannel").(string)

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

	// If the payment channel account is set, we'll use it, otherwise the source account is to be used
	var paymentChannelAccount *Account
	if paymentChannel != "" {
		paymentChannelAccount, err = b.readVaultAccount(ctx, req, "accounts/"+paymentChannel)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		paymentChannelAccount = sourceAccount
	}
	paymentChannelAddress := paymentChannelAccount.Address

	// Build the transaction
	// TODO: multisig
	// TODO: custom assets
	tx, err := build.Transaction(
		build.SourceAccount{AddressOrSeed: paymentChannelAddress},
		build.TestNetwork,
		build.AutoSequence{SequenceProvider: horizon.DefaultTestNetClient},
		build.Payment(
			build.SourceAccount{AddressOrSeed: sourceAddress},
			build.Destination{AddressOrSeed: destinationAddress},
			build.NativeAmount{Amount: amount},
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Sign the transaction with the necessary signatures
	var signers []string
	signers = append(signers, sourceAccount.Seed)
	if paymentChannel != "" {
		signers = append(signers, paymentChannelAccount.Seed)
	}
	signedTx, err := tx.Sign(signers...)
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
