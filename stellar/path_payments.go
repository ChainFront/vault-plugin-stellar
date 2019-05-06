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
	"github.com/pkg/errors"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/keypair"
	"math/big"
	"strings"
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
					Description: "(Optional) Payment channel account",
				},
				"additionalSigners": &framework.FieldSchema{
					Type:        framework.TypeCommaStringSlice,
					Description: "(Optional) Array of additional signers for this transaction",
				},
				"amount": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Amount to send",
				},
				"assetCode": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "Code of asset to send (use 'native' for XLM)",
				},
				"assetIssuer": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "(Optional) If paying with a non-native asset, this is the issuer address",
				},
				"memo": &framework.FieldSchema{
					Type:        framework.TypeString,
					Description: "(Optional) An optional memo to include with the payment transaction",
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
		return nil, logical.CodedError(400, err.Error())
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

	amountStr := d.Get("amount").(string)
	if amountStr == "" {
		return errMissingField("amount"), nil
	}
	amount := validNumber(amountStr)

	assetCode := d.Get("assetCode").(string)
	if assetCode == "" {
		return errMissingField("assetCode"), nil
	}

	// Read optional fields
	paymentChannel := d.Get("paymentChannel").(string)

	// Read optional fields
	assetIssuer := d.Get("assetIssuer").(string)
	if assetIssuer == "" && !strings.EqualFold(assetCode, "native") {
		return errMissingField("assetIssuer"), nil
	}

	// Read the optional additionalSigners field
	var additionalSigners []string
	if additionalSignersRaw, ok := d.GetOk("additionalSigners"); ok {
		additionalSigners = additionalSignersRaw.([]string)
	}

	// Read the optional memo field
	memo := d.Get("memo").(string)

	// Retrieve the source account keypair from vault storage
	sourceAccount, err := b.readVaultAccount(ctx, req, "accounts/"+source)
	if err != nil {
		return nil, err
	}
	if sourceAccount == nil {
		return nil, logical.CodedError(400, "source account not found")
	}
	sourceAddress := sourceAccount.Address

	// Retrieve the destination account keypair from vault storage
	destinationAccount, err := b.readVaultAccount(ctx, req, "accounts/"+destination)
	if err != nil {
		return nil, err
	}
	if destinationAccount == nil {
		return nil, logical.CodedError(400, "destination account not found")
	}
	destinationAddress := destinationAccount.Address

	// If the payment channel account is set, we'll use it, otherwise the source account is to be used
	var paymentChannelAccount *Account
	if paymentChannel != "" {
		paymentChannelAccount, err = b.readVaultAccount(ctx, req, "accounts/"+paymentChannel)
		if err != nil {
			return nil, err
		}
		if paymentChannelAccount == nil {
			return nil, logical.CodedError(400, "payment channel account not found")
		}
	} else {
		paymentChannelAccount = sourceAccount
	}
	paymentChannelAddress := paymentChannelAccount.Address

	// If additionalSigners is set, look up all the keys for these accounts
	var additionalSignerAccounts []Account
	for _, additionalSigner := range additionalSigners {
		additionalSignerAccount, err := b.readVaultAccount(ctx, req, "accounts/"+additionalSigner)
		if err != nil {
			return nil, err
		}
		if additionalSignerAccount == nil {
			return nil, logical.CodedError(400, "additional signer account not found: "+additionalSigner)
		}
		additionalSignerAccounts = append(additionalSignerAccounts, *additionalSignerAccount)
	}

	// Validate that this transaction is allowed given the constraints on the source account (whitelist, blacklist, spend limit)
	if valid, err := b.validAccountConstraints(sourceAccount, amount, destinationAddress); !valid {
		return nil, err
	}

	// Build the payment object depending on what type of asset we're using
	var payment build.PaymentBuilder
	if strings.EqualFold(assetCode, "native") {
		payment = build.Payment(
			build.SourceAccount{AddressOrSeed: sourceAddress},
			build.Destination{AddressOrSeed: destinationAddress},
			build.NativeAmount{Amount: amount.String()},
		)
	} else {
		// Validate that issuer is a proper stellar address
		_, err := keypair.Parse(assetIssuer)
		if err != nil {
			return nil, logical.CodedError(400, "invalid address for assetIssuer")
		}

		payment = build.Payment(
			build.SourceAccount{AddressOrSeed: sourceAddress},
			build.Destination{AddressOrSeed: destinationAddress},
			build.CreditAmount{Code: assetCode, Issuer: assetIssuer, Amount: amount.String()},
		)
	}

	// Build the base transaction
	tx, err := build.Transaction(
		build.SourceAccount{AddressOrSeed: paymentChannelAddress},
		build.TestNetwork,
		build.AutoSequence{SequenceProvider: horizon.DefaultTestNetClient},
		build.MemoText{Value: memo},
		payment,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build payment object")
	}

	// Build up our array of signer private keys
	var signers []string
	signers = append(signers, sourceAccount.Seed)
	if paymentChannel != "" {
		signers = append(signers, paymentChannelAccount.Seed)
	}

	// Stellar currently rejects transactions with more signers than expected based on the account thresholds. So we
	// comment these signatures for now. https://github.com/stellar/stellar-protocol/issues/120
	//for _, additionalSigner := range additionalSignerAccounts {
	//	signers = append(signers, additionalSigner.Seed)
	//}

	// Sign the transaction with the necessary signatures (source, paymentChannel, additionalSigners)
	signedTx, err := tx.Sign(signers...)
	if err != nil {
		return nil, err
	}

	// Convert to base64
	signedTxBase64, err := signedTx.Base64()
	if err != nil {
		return nil, err
	}

	// Get the transaction hash
	txHash, err := tx.HashHex()
	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"source_address":     tx.TX.SourceAccount.Address(),
			"account_sequence":   tx.TX.SeqNum,
			"fee":                tx.TX.Fee,
			"transaction_hash":   txHash,
			"signed_transaction": signedTxBase64,
		},
	}, nil
}

func (b *backend) validAccountConstraints(account *Account, amount *big.Int, toAddress string) (bool, error) {
	txLimit := validNumber(account.TxSpendLimit)

	if txLimit.Cmp(amount) == -1 && txLimit.Cmp(big.NewInt(0)) == 1 {
		return false, fmt.Errorf("transaction amount (%s) is larger than the transactional limit (%s)", amount.String(), account.TxSpendLimit)
	}

	if contains(account.Blacklist, toAddress) {
		return false, fmt.Errorf("%s is blacklisted", toAddress)
	}

	if len(account.Whitelist) > 0 && !contains(account.Whitelist, toAddress) {
		return false, fmt.Errorf("%s is not in the whitelist", toAddress)
	}

	return true, nil
}
