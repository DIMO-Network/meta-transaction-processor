package ticker

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/internal/mocks"
	"github.com/DIMO-Network/meta-transaction-processor/internal/models"
	"github.com/DIMO-Network/meta-transaction-processor/internal/sender"
	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/meta-transaction-processor/internal/testcontract"
	"github.com/DIMO-Network/shared/db"
	"github.com/docker/go-connections/nat"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"go.uber.org/mock/gomock"
)

type ProcessorTestSuite struct {
	suite.Suite

	pgCont *postgres.PostgresContainer
	dbs    db.Store
	// mockCtrl   *gomock.Controller
	// dexClient  *mocks.MockDexClient
	// gokaTester *tester.Tester
}

func TestProcessorTestSuite(t *testing.T) {
	suite.Run(t, new(ProcessorTestSuite))
}

func (s *ProcessorTestSuite) createAccount() (common.Address, *ecdsa.PrivateKey) {
	sk, err := crypto.GenerateKey()
	s.Require().NoError(err)

	pk, _ := sk.Public().(*ecdsa.PublicKey)

	addr := crypto.PubkeyToAddress(*pk)

	return addr, sk
}

var eth = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

func (s *ProcessorTestSuite) TestSubmitNew() {
	ctx := context.Background()

	container, err := postgres.RunContainer(
		ctx,
		testcontainers.WithImage("docker.io/postgres:12.9-alpine"),
		postgres.WithDatabase("meta_transaction_processor"),
		postgres.WithUsername("dimo"),
		postgres.WithPassword("dimo"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	s.Require().NoError(err)

	s.pgCont = container

	h, err := container.Host(ctx)
	s.Require().NoError(err)

	p, err := container.MappedPort(ctx, nat.Port("5432/tcp"))
	s.Require().NoError(err)

	settings := db.Settings{
		User:               "dimo",
		Password:           "dimo",
		Port:               p.Port(),
		Host:               h,
		Name:               "meta_transaction_processor",
		MaxOpenConnections: 2,
		MaxIdleConnections: 2,
	}

	dbs := db.NewDbConnectionFromSettings(ctx, &settings, false)
	for !dbs.IsReady() {
		time.Sleep(500 * time.Millisecond)
	}

	s.dbs = dbs

	_, err = dbs.DBS().Writer.Exec(`CREATE SCHEMA IF NOT EXISTS meta_transaction_processor;`)
	s.Require().NoError(err)

	goose.SetTableName("meta_transaction_processor.migrations")
	err = goose.RunContext(ctx, "up", dbs.DBS().Writer.DB, "../../migrations")
	s.Require().NoError(err)

	// TODO(elffjs): Why doesn't Snapshot/Restore work?
	err = container.Snapshot(ctx)
	s.Require().NoError(err)

	logger := zerolog.Nop()

	deployAddr, deploySK := s.createAccount()

	relayAddr, relaySK := s.createAccount()

	backend := simulated.NewBackend(types.GenesisAlloc{
		relayAddr: types.Account{
			Balance: new(big.Int).Mul(big.NewInt(10_000), eth),
		},
		deployAddr: types.Account{
			Balance: new(big.Int).Mul(big.NewInt(10_000), eth),
		},
	})
	client := backend.Client()

	chainID, err := client.ChainID(ctx)
	s.Require().NoError(err)

	auth, err := bind.NewKeyedTransactorWithChainID(deploySK, chainID)
	s.Require().NoError(err)

	contractAddr, _, _, err := testcontract.DeployTestcontract(auth, client)
	s.Require().NoError(err)

	backend.Commit()

	sender, _ := sender.FromKey(hexutil.Encode(crypto.FromECDSA(relaySK)))

	mockCtrl := gomock.NewController(s.T())
	producer := mocks.NewMockProducer(mockCtrl)

	w := Watcher{
		logger:             &logger,
		confirmationBlocks: big.NewInt(3),
		boostAfterBlocks:   big.NewInt(10),
		prod:               producer,
		dbs:                s.dbs,
		client:             client,
		sender:             sender,
		chainID:            big.NewInt(1337),
		walletIndex:        2,
	}

	mtr := models.MetaTransactionRequest{
		ID:          ksuid.New().String(),
		To:          contractAddr.Bytes(),
		WalletIndex: 2,
		Data:        common.FromHex("0x7050f4c0"),
	}

	var txHash common.Hash
	producer.EXPECT().Submitted(gomock.Any()).DoAndReturn(func(msg *status.SubmittedMsg) {
		s.Require().Equal(mtr.ID, msg.ID)
		txHash = msg.Hash
	})

	err = mtr.Insert(ctx, s.dbs.DBS().Writer, boil.Infer())
	s.Require().NoError(err)

	backend.Commit()

	submissionBlock, err := client.BlockByNumber(ctx, nil)
	s.Require().NoError(err)

	err = w.Tick(ctx)
	s.Require().NoError(err)

	backend.Commit()
	_, pending, err := client.TransactionByHash(ctx, txHash)
	s.Require().NoError(err)

	s.False(pending)

	err = mtr.Reload(ctx, s.dbs.DBS().Reader)
	s.Require().NoError(err)

	s.Equal(big.NewInt(2), mtr.SubmittedBlockNumber.Int(nil))
	s.Equal(submissionBlock.Hash().Bytes(), mtr.SubmittedBlockHash.Bytes)

	backend.Commit()

	producer.EXPECT().Mined(&status.MinedMsg{
		ID:   mtr.ID,
		Hash: txHash,
	})

	err = w.Tick(ctx)
	s.Require().NoError(err)

	err = mtr.Reload(ctx, dbs.DBS().Reader)
	s.Require().NoError(err)

	s.Equal(big.NewInt(3), mtr.MinedBlockNumber.Int(nil))

	backend.Commit()
	err = w.Tick(ctx)
	s.Require().NoError(err)

	producer.EXPECT().Confirmed(gomock.Any())

	backend.Commit()
	err = w.Tick(ctx)
	s.Require().NoError(err)
}
