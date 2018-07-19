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

func TestBackend_createPayment(t *testing.T) {

	td := setupTest(t)
	createAccount(td, "testSourceAccount", t)
	createAccount(td, "testDestinationAccount", t)

	createPayment(td, "testSourceAccount", "testDestinationAccount", "35", t)

	//
	//response, err := td.Client.SubmitTransaction(signedTx)
	//if err != nil {
	//	t.Fatalf("failed to submit transaction to testnet: %v", err)
	//}
}

func createAccount(td *testData, accountName string, t *testing.T) {
	d :=
		map[string]interface{}{
			"xlm_balance": "50",
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

func createPayment(td *testData, sourceAccountName string, destinationAccountName string, amount string, t *testing.T) {
	d :=
		map[string]interface{}{
			"source":      sourceAccountName,
			"destination": destinationAccountName,
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
}
