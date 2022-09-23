package sender

import (
	"context"
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type keySender struct {
	privateKey *ecdsa.PrivateKey
	address    common.Address
}

func (a *keySender) Address() common.Address {
	return a.address
}

func (a *keySender) Sign(_ context.Context, hash common.Hash) ([]byte, error) {
	return crypto.Sign(hash[:], a.privateKey)
}

func FromKey(hexPriv string) (Sender, error) {
	privBytes := common.FromHex(hexPriv)
	priv, err := crypto.ToECDSA(privBytes)
	if err != nil {
		return nil, err
	}

	address := crypto.PubkeyToAddress(priv.PublicKey)

	return &keySender{privateKey: priv, address: address}, nil
}
