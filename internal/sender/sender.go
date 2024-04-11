package sender

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
)

type Sender interface {
	// Address returns the Ethereum address for the signer.
	Address() common.Address
	// Sign takes in a hash and signs it using the sender's private key.
	Sign(ctx context.Context, hash common.Hash) ([]byte, error)
}
