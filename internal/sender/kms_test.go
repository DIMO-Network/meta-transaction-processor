package sender

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/docker/go-connections/nat"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
)

// Tough to test V byte correction.
func TestKMSSenderSign(t *testing.T) {
	ctx := context.Background()

	localstackContainer, err := localstack.RunContainer(ctx,
		testcontainers.WithImage("localstack/localstack:latest"),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	defer func() {
		if err := localstackContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate container: %s", err)
		}
	}()

	mappedPort, err := localstackContainer.MappedPort(ctx, nat.Port("4566/tcp"))
	if err != nil {
		t.Fatal()
	}

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		t.Fatal()
	}
	defer provider.Close()

	host, err := provider.DaemonHost(ctx)
	if err != nil {
		t.Fatal()
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, opts ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: fmt.Sprintf("http://%s:%d", host, mappedPort.Int()),
			}, nil
		})

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		t.Fatal()
	}

	kmsClient := kms.NewFromConfig(awsCfg)

	createOut, err := kmsClient.CreateKey(ctx, &kms.CreateKeyInput{
		KeySpec:  types.KeySpecEccSecgP256k1,
		KeyUsage: types.KeyUsageTypeSignVerify,
	})
	if err != nil {
		t.Fatal()
	}

	keyID := *createOut.KeyMetadata.KeyId

	sender, err := FromKMS(ctx, kmsClient, keyID)
	if err != nil {
		t.Fatal(err)
	}

	hashSlice := make([]byte, 32)
	_, err = rand.Read(hashSlice)
	if err != nil {
		t.Fatal(err)
	}
	hash := common.BytesToHash(hashSlice)

	signature, err := sender.Sign(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}

	if len(signature) != 65 {
		t.Fatalf("signature has length %d instead of the expected 65", len(signature))
	}

	recPub, err := crypto.Ecrecover(hash[:], signature)
	if err != nil {
		t.Fatal(err)
	}

	pub, err := crypto.UnmarshalPubkey(recPub)
	if err != nil {
		t.Fatal(err)
	}

	addr := crypto.PubkeyToAddress(*pub)

	if addr != sender.Address() {
		t.Fatal("signature did not recover to the address")
	}
}
