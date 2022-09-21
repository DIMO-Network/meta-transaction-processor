package manager

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Sender interface {
	Address() common.Address
	Sign(common.Hash) ([]byte, error)
}

type keyAccount struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
}

func (a *keyAccount) Address() common.Address {
	return a.address
}

func (a *keyAccount) Sign(hash common.Hash) ([]byte, error) {
	return crypto.Sign(hash[:], a.privateKey)
}

func KeyAccount(hexPrivateKey string) (Sender, error) {
	private, err := crypto.HexToECDSA(hexPrivateKey)
	if err != nil {
		return nil, err
	}

	address := crypto.PubkeyToAddress(private.PublicKey)

	return &keyAccount{privateKey: private, address: address}, nil
}
