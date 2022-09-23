package manager

import (
	"context"
	"math/big"

	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/meta-transaction-processor/internal/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

type TransactionRequest struct {
	ID   string
	To   common.Address
	Data []byte
}

type Manager interface {
	SendTx(ctx context.Context, req *TransactionRequest) error
	Receipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

func New(client *ethclient.Client, chainID *big.Int, sender Sender, storage storage.Storage, logger *zerolog.Logger, sprod status.Producer) Manager {
	return &manager{
		client:   client,
		chainID:  chainID,
		sender:   sender,
		storage:  storage,
		logger:   logger,
		producer: sprod,
	}
}

type manager struct {
	chainID  *big.Int
	sender   Sender
	client   *ethclient.Client
	storage  storage.Storage
	logger   *zerolog.Logger
	producer status.Producer
}

func (m *manager) SendTx(ctx context.Context, req *TransactionRequest) error {
	head, err := m.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	nonce, err := m.client.PendingNonceAt(ctx, m.sender.Address())
	if err != nil {
		return err
	}

	signer := types.LatestSignerForChainID(m.chainID)

	gasPrice, err := m.client.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}

	callMsg := ethereum.CallMsg{
		From:     m.sender.Address(),
		To:       &req.To,
		GasPrice: gasPrice,
		Data:     req.Data,
	}

	gasLimit, err := m.client.EstimateGas(ctx, callMsg)
	if err != nil {
		return err
	}

	txd := &types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       &req.To,
		Data:     req.Data,
	}

	tx := types.NewTx(txd)

	sigHash := signer.Hash(tx)
	sigBytes, err := m.sender.Sign(sigHash)
	if err != nil {
		return err
	}
	signedTx, err := tx.WithSignature(signer, sigBytes)
	if err != nil {
		return err
	}
	txHash := signedTx.Hash()

	store := &storage.Transaction{
		ID:   req.ID,
		To:   req.To,
		Data: req.Data,

		Nonce:    nonce,
		GasPrice: gasPrice,
		Hash:     txHash,

		CreationBlock: &storage.Block{
			Number:    head.Number,
			Hash:      head.Hash(),
			Timestamp: head.Time,
		},
	}

	m.logger.Info().Str("id", req.ID).Str("hash", txHash.String()).Msg("Sending transaction.")

	err = m.storage.New(store)
	if err != nil {
		return err
	}

	err = m.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return err
	}

	m.producer.Created(&status.CreatedMsg{
		ID:    req.ID,
		Hash:  txHash,
		Block: store.CreationBlock,
	})

	return nil
}

func (m *manager) Receipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return m.client.TransactionReceipt(ctx, txHash)
}
