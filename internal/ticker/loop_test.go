package ticker

import (
	"context"
	"crypto/ecdsa"
	"fmt"
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

type WatcherTestSuite struct {
	suite.Suite

	pgCont *postgres.PostgresContainer
	dbs    db.Store

	relayAddr common.Address
	relaySK   *ecdsa.PrivateKey

	contractAddr common.Address

	w        Watcher
	producer *mocks.MockProducer

	client  simulated.Client
	backend *simulated.Backend
}

func TestWatcherTestSuite(t *testing.T) {
	suite.Run(t, new(WatcherTestSuite))
}

func (s *WatcherTestSuite) createAccount() (common.Address, *ecdsa.PrivateKey) {
	sk, err := crypto.GenerateKey()
	s.Require().NoError(err)

	pk, _ := sk.Public().(*ecdsa.PublicKey)

	addr := crypto.PubkeyToAddress(*pk)

	return addr, sk
}

var eth = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

func (s *WatcherTestSuite) SetupSuite() {
	ctx := context.Background()

	container, err := postgres.Run(
		ctx,
		"docker.io/postgres:16.6-alpine",
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

	s.relayAddr, s.relaySK = s.createAccount()
}

func (s *WatcherTestSuite) SetupTest() {
	ctx := context.Background()

	_, err := models.MetaTransactionRequests().DeleteAll(ctx, s.dbs.DBS().Writer)
	s.Require().NoError(err)

	logger := zerolog.Nop()

	deployAddr, deploySK := s.createAccount()

	s.backend = simulated.NewBackend(types.GenesisAlloc{
		s.relayAddr: types.Account{
			Balance: new(big.Int).Mul(big.NewInt(10_000), eth),
		},
		deployAddr: types.Account{
			Balance: new(big.Int).Mul(big.NewInt(10_000), eth),
		},
	})

	s.client = s.backend.Client()

	chainID, err := s.client.ChainID(ctx)
	s.Require().NoError(err)

	auth, err := bind.NewKeyedTransactorWithChainID(deploySK, chainID)
	s.Require().NoError(err)

	s.contractAddr, _, _, err = testcontract.DeployTestcontract(auth, s.client)
	s.Require().NoError(err)

	s.backend.Commit()

	sender, _ := sender.FromKey(hexutil.Encode(crypto.FromECDSA(s.relaySK)))

	mockCtrl := gomock.NewController(s.T())
	s.producer = mocks.NewMockProducer(mockCtrl)

	s.w = Watcher{
		logger:             &logger,
		confirmationBlocks: big.NewInt(3),
		boostAfterBlocks:   big.NewInt(10),
		prod:               s.producer,
		dbs:                s.dbs,
		client:             s.client,
		sender:             sender,
		chainID:            big.NewInt(1337),
		walletIndex:        2,
	}
}

func (s *WatcherTestSuite) TestSubmitSuccess() {
	ctx := context.Background()

	mtr := models.MetaTransactionRequest{
		ID:          ksuid.New().String(),
		To:          s.contractAddr.Bytes(),
		WalletIndex: 2,
		Data:        common.FromHex("0x7050f4c0"),
	}

	subCapt := &ArgCaptor[*status.SubmittedMsg]{}

	s.producer.EXPECT().Submitted(subCapt)

	err := mtr.Insert(ctx, s.dbs.DBS().Writer, boil.Infer())
	s.Require().NoError(err)

	s.backend.Commit()

	submissionBlock, err := s.client.BlockByNumber(ctx, nil)
	s.Require().NoError(err)

	err = s.w.Tick(ctx)
	s.Require().NoError(err)

	s.Require().Equal(mtr.ID, subCapt.Value().ID)
	txHash := subCapt.Value().Hash

	s.backend.Commit()
	_, pending, err := s.client.TransactionByHash(ctx, txHash)
	s.Require().NoError(err)

	s.False(pending)

	err = mtr.Reload(ctx, s.dbs.DBS().Reader)
	s.Require().NoError(err)

	s.Equal(big.NewInt(2), mtr.SubmittedBlockNumber.Int(nil))
	s.Equal(submissionBlock.Hash().Bytes(), mtr.SubmittedBlockHash.Bytes)

	s.backend.Commit()

	s.producer.EXPECT().Mined(&status.MinedMsg{
		ID:   mtr.ID,
		Hash: txHash,
	})

	err = s.w.Tick(ctx)
	s.Require().NoError(err)

	err = mtr.Reload(ctx, s.dbs.DBS().Reader)
	s.Require().NoError(err)

	s.Equal(big.NewInt(3), mtr.MinedBlockNumber.Int(nil))

	s.backend.Commit()
	err = s.w.Tick(ctx)
	s.Require().NoError(err)

	s.producer.EXPECT().Confirmed(&status.ConfirmedMsg{
		ID:         mtr.ID,
		Hash:       txHash,
		Logs:       make([]*status.Log, 0),
		Successful: true,
	})

	s.backend.Commit()
	err = s.w.Tick(ctx)
	s.Require().NoError(err)
}

func (s *WatcherTestSuite) TestSubmitCustomErrorWithArgs() {
	ctx := context.Background()

	mtr := models.MetaTransactionRequest{
		ID:          ksuid.New().String(),
		To:          s.contractAddr.Bytes(),
		WalletIndex: 2,
		Data:        common.FromHex("0x4740a9ce"),
	}

	abi, _ := testcontract.TestcontractMetaData.GetAbi()
	b, err := abi.Errors["ErrorOneArg"].Inputs.Pack(big.NewInt(42))
	s.Require().NoError(err)

	b = append(abi.Errors["ErrorOneArg"].ID.Bytes()[:4], b...)

	s.producer.EXPECT().Failed(&status.FailedMsg{
		ID:   mtr.ID,
		Data: b,
	})

	err = mtr.Insert(ctx, s.dbs.DBS().Writer, boil.Infer())
	s.Require().NoError(err)

	err = s.w.Tick(ctx)
	s.Require().NoError(err)
}

type ArgCaptor[A any] struct {
	value A
}

func (m *ArgCaptor[A]) Matches(x any) bool {
	// Not thread safe!
	if a, ok := x.(A); ok {
		m.value = a
		return true
	}
	return false
}

func (m *ArgCaptor[A]) String() string {
	return fmt.Sprintf("Matches and captures a value of type %T", m.value)
}

func (m *ArgCaptor[A]) Value() A {
	return m.value
}
