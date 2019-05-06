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
	"testing"
	"time"

	"fmt"
	"github.com/hashicorp/vault/logical"
	"github.com/stellar/go/clients/horizon"
)

const (
	defaultLeaseTTLHr = 1
	maxLeaseTTLHr     = 12
)

// Set up/Teardown
type testData struct {
	B      logical.Backend
	S      logical.Storage
	Client horizon.Client
}

func setupTest(t *testing.T) *testData {
	horizonClient := *horizon.DefaultTestNetClient
	b, reqStorage := getTestBackend(t)
	return &testData{
		B:      b,
		S:      reqStorage,
		Client: horizonClient,
	}
}

func getTestBackend(t *testing.T) (logical.Backend, logical.Storage) {
	b := Backend()

	config := &logical.BackendConfig{
		System: &logical.StaticSystemView{
			DefaultLeaseTTLVal: defaultLeaseTTLHr * time.Hour,
			MaxLeaseTTLVal:     maxLeaseTTLHr * time.Hour,
		},
		StorageView: &logical.InmemStorage{},
	}
	err := b.Setup(context.Background(), config)
	if err != nil {
		t.Fatalf("unable to create backend: %v", err)
	}

	return b, config.StorageView
}

func TestBackend_createAccount(t *testing.T) {

	td := setupTest(t)

	accountName := "account1"
	createAccount(td, accountName, t)
}

func TestBackend_submitPayment(t *testing.T) {

	td := setupTest(t)
	createAccount(td, "testSourceAccount", t)
	createAccount(td, "testDestinationAccount", t)

	respData := createPayment(td, "testSourceAccount", "testDestinationAccount", "35", t)

	signedTx, ok := respData["signed_transaction"]
	if !ok {
		t.Fatalf("expected signedTx data not present in createPayment")
	}

	response, err := td.Client.SubmitTransaction(signedTx.(string))
	if err != nil {
		t.Fatalf("failed to submit transaction to testnet: %v", errorString(err))
	}

	t.Logf("transaction posted in ledger: %v", response.Ledger)
}

func TestBackend_submitPaymentAboveLimit(t *testing.T) {

	td := setupTest(t)
	createAccount(td, "testSourceAccount", t)
	createAccount(td, "testDestinationAccount", t)

	respData := createPayment(td, "testSourceAccount", "testDestinationAccount", "1001", t)

	signedTx, ok := respData["signed_transaction"]
	if !ok {
		t.Fatalf("expected signedTx data not present in createPayment")
	}

	response, err := td.Client.SubmitTransaction(signedTx.(string))
	if err != nil {
		t.Fatalf("failed to submit transaction to testnet: %v", errorString(err))
	}

	t.Logf("transaction posted in ledger: %v", response.Ledger)
}

func TestBackend_submitPaymentUsingChannel(t *testing.T) {

	td := setupTest(t)
	createAccount(td, "testSourceAccount", t)
	createAccount(td, "testDestinationAccount", t)
	createAccount(td, "testPaymentChannelAccount", t)

	respData := createPaymentWithChannel(td, "testSourceAccount", "testDestinationAccount", "testPaymentChannelAccount", "35", t)

	signedTx, ok := respData["signed_transaction"]
	if !ok {
		t.Fatalf("expected signedTx data not present in createPayment")
	}

	response, err := td.Client.SubmitTransaction(signedTx.(string))
	if err != nil {
		t.Fatalf("failed to submit transaction to testnet: %v", errorString(err))
	}

	t.Logf("transaction posted in ledger: %v", response.Ledger)
}

func TestBackend_submitPaymentUsingChannelAndAdditionalSigners(t *testing.T) {

	td := setupTest(t)
	createAccount(td, "testSourceAccount", t)
	createAccount(td, "testDestinationAccount", t)
	createAccount(td, "testPaymentChannelAccount", t)
	createAccount(td, "testAdditionalSigner1Account", t)
	createAccount(td, "testAdditionalSigner2Account", t)

	var additionalSigners []string
	additionalSigners = append(additionalSigners, "testAdditionalSigner1Account", "testAdditionalSigner2Account")

	respData := createPaymentWithChannelAndAdditionalSigners(td, "testSourceAccount", "testDestinationAccount", "testPaymentChannelAccount", additionalSigners, "35", t)

	signedTx, ok := respData["signed_transaction"]
	if !ok {
		t.Fatalf("expected signedTx data not present in createPayment")
	}

	response, err := td.Client.SubmitTransaction(signedTx.(string))
	if err != nil {
		t.Fatalf("failed to submit transaction to testnet: %v", errorString(err))
	}

	t.Logf("transaction posted in ledger: %v", response.Ledger)
}

func createAccount(td *testData, accountName string, t *testing.T) {
	d :=
		map[string]interface{}{
			"xlm_balance":    "50",
			"tx_spend_limit": "1000",
		}
	resp, err := td.B.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.CreateOperation,
		Path:      fmt.Sprintf("accounts/%s", accountName),
		Data:      d,
		Storage:   td.S,
	})
	if err != nil {
		t.Fatalf("failed to create account: %v", err)
	}
	if resp.IsError() {
		t.Fatal(resp.Error())
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	t.Log(resp.Data)
}

func createPayment(td *testData, sourceAccountName string, destinationAccountName string, amount string, t *testing.T) map[string]interface{} {
	d :=
		map[string]interface{}{
			"source":      sourceAccountName,
			"destination": destinationAccountName,
			"assetCode":   "native",
			"amount":      amount,
		}
	resp, err := td.B.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.CreateOperation,
		Path:      fmt.Sprintf("payments"),
		Data:      d,
		Storage:   td.S,
	})
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}
	if resp.IsError() {
		t.Fatal(resp.Error())
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	t.Log(resp.Data)

	return resp.Data
}

func createPaymentWithChannel(td *testData, sourceAccountName string, destinationAccountName string, paymentChannelAccountName string, amount string, t *testing.T) map[string]interface{} {
	d :=
		map[string]interface{}{
			"source":         sourceAccountName,
			"destination":    destinationAccountName,
			"paymentChannel": paymentChannelAccountName,
			"assetCode":      "native",
			"amount":         amount,
		}
	resp, err := td.B.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.CreateOperation,
		Path:      fmt.Sprintf("payments"),
		Data:      d,
		Storage:   td.S,
	})
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}
	if resp.IsError() {
		t.Fatal(resp.Error())
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	t.Log(resp.Data)

	return resp.Data
}

func createPaymentWithChannelAndAdditionalSigners(td *testData, sourceAccountName string, destinationAccountName string, paymentChannelAccountName string, additionalSigners []string, amount string, t *testing.T) map[string]interface{} {
	d :=
		map[string]interface{}{
			"source":            sourceAccountName,
			"destination":       destinationAccountName,
			"paymentChannel":    paymentChannelAccountName,
			"additionalSigners": additionalSigners,
			"assetCode":         "native",
			"amount":            amount,
		}
	resp, err := td.B.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.CreateOperation,
		Path:      fmt.Sprintf("payments"),
		Data:      d,
		Storage:   td.S,
	})
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}
	if resp.IsError() {
		t.Fatal(resp.Error())
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	t.Log(resp.Data)

	return resp.Data
}
