package stellar

import (
	"github.com/stellar/go/keypair"
	"log"
	"testing"
)

func TestSum(t *testing.T) {

	random, err := keypair.Random()
	if err != nil {
		log.Fatal(err)
	}

	address := random.Address()
	seed := random.Seed()

	log.Print("Public key : " + address + "  -- Private key : " + seed)

}
