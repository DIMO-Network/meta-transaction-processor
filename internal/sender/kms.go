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
	keyID     string
	publicKey []byte
	address   common.Address
	client    *kms.Client
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

	signOutput, err := s.client.Sign(ctx, signInput)
	if err != nil {
		return nil, err
	}

	var signOutputSig ecdsaSigValue
	_, err = asn1.Unmarshal(signOutput.Signature, &signOutputSig)
	if err != nil {
		return nil, err
	}

	// Correct S, if necessary, so that it's in the lower half of the group.
	sigS := signOutputSig.S
	if sigS.Cmp(secp256k1HalfN) > 0 {
		sigS = new(big.Int).Sub(secp256k1N, sigS)
	}

	sigRSV := make([]byte, 65)
	signOutputSig.R.FillBytes(sigRSV[:32])
	sigS.FillBytes(sigRSV[32:64])

	recPub, err := crypto.Ecrecover(hash[:], sigRSV)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(recPub, s.publicKey) {
		return sigRSV, nil
	}

	sigRSV[64] = 1
	recPub, err = crypto.Ecrecover(hash[:], sigRSV)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(recPub, s.publicKey) {
		return sigRSV, nil
	}

	return nil, fmt.Errorf("couldn't choose a working V from the returned R and S")
}

// subjectPublicKeyInfo respresents the PublicKey field on the KMS GetPublicKey response.
// See https://docs.aws.amazon.com/kms/latest/APIReference/API_GetPublicKey.html
type subjectPublicKeyInfo struct {
	Algorithm        pkix.AlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

// ecdsaSigValue represents the Signature field on the KMS Sign response.
// See https://docs.aws.amazon.com/kms/latest/APIReference/API_Sign.html#API_Sign_ResponseSyntax
type ecdsaSigValue struct {
	R *big.Int
	S *big.Int
}

func FromKMS(ctx context.Context, client *kms.Client, keyID string) (Sender, error) {
	pubResp, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{KeyId: aws.String(keyID)})
	if err != nil {
		return nil, err
	}

	var spki subjectPublicKeyInfo

	_, err = asn1.Unmarshal(pubResp.PublicKey, &spki)
	if err != nil {
		return nil, err
	}

	pub, err := crypto.UnmarshalPubkey(spki.SubjectPublicKey.Bytes)
	if err != nil {
		return nil, err
	}

	pubBytes := secp256k1.S256().Marshal(pub.X, pub.Y)

	addr := crypto.PubkeyToAddress(*pub)

	return &kmsSender{keyID: keyID, publicKey: pubBytes, address: addr, client: client}, nil
}
