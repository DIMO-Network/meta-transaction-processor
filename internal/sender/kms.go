package sender

import (
	"bytes"
	"context"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"math/big"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

var secp256k1N = crypto.S256().Params().N
var secp256k1HalfN = new(big.Int).Div(secp256k1N, big.NewInt(2))

type kmsSender struct {
	keyID   string
	pub     []byte
	address common.Address
	client  *kms.Client
}

func (s *kmsSender) Address() common.Address {
	return s.address
}

func (s *kmsSender) Sign(ctx context.Context, hash common.Hash) ([]byte, error) {
	signInput := &kms.SignInput{
		KeyId:            aws.String(s.keyID),
		SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
		MessageType:      types.MessageTypeDigest,
		Message:          hash[:],
	}

	out, err := s.client.Sign(ctx, signInput)
	if err != nil {
		return nil, err
	}

	var sigVal ecdsaSigValue
	_, err = asn1.Unmarshal(out.Signature, &sigVal)
	if err != nil {
		return nil, err
	}

	sigS := sigVal.S

	if sigS.Cmp(secp256k1HalfN) > 0 {
		sigS = new(big.Int).Sub(secp256k1N, sigS)
	}

	fullSig := make([]byte, 65)

	// Determine whether V ought to be 0 or 1.
	sigVal.R.FillBytes(fullSig[:32])
	sigS.FillBytes(fullSig[32:64])

	recPub, err := crypto.Ecrecover(hash[:], fullSig)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(recPub, s.pub) {
		return fullSig, nil
	}

	fullSig[64] = 1
	recPub, err = crypto.Ecrecover(hash[:], fullSig)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(recPub, s.pub) {
		return fullSig, nil
	}

	return nil, fmt.Errorf("couldn't choose a working V from the returned R and S")
}

// See https://datatracker.ietf.org/doc/html/rfc5280#section-4.1
type subjectPublicKeyInfo struct {
	Algorithm        pkix.AlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

// See https://datatracker.ietf.org/doc/html/rfc3279#section-2.2.3
type ecdsaSigValue struct {
	R *big.Int
	S *big.Int
}

func FromKMS(ctx context.Context, client *kms.Client, keyID string) (Sender, error) {
	pubResp, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{KeyId: aws.String(keyID)})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve public key: %w", err)
	}

	var pki subjectPublicKeyInfo
	_, err = asn1.Unmarshal(pubResp.PublicKey, &pki)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal DER: %w", err)
	}

	pub, err := crypto.UnmarshalPubkey(pki.SubjectPublicKey.Bytes)
	if err != nil {
		return nil, err
	}

	pubBytes := secp256k1.S256().Marshal(pub.X, pub.Y)

	addr := crypto.PubkeyToAddress(*pub)

	return &kmsSender{keyID: keyID, pub: pubBytes, address: addr, client: client}, nil
}
