package manager

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/DIMO-Network/meta-transaction-processor/internal/sender"
	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/meta-transaction-processor/internal/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

type TransactionRequest struct {
	ID    string
	To    common.Address
	Data  []byte
	Nonce *uint64
}

type Manager interface {
	SendTx(ctx context.Context, req *TransactionRequest) error
	Receipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	Head(ctx context.Context) (*types.Header, error)
}

func New(client *ethclient.Client, chainID *big.Int, sender sender.Sender, storage storage.Storage, logger *zerolog.Logger, sprod status.Producer, gasPriceFactor *big.Rat) Manager {
	return &manager{
		client:         client,
		chainID:        chainID,
		sender:         sender,
		storage:        storage,
		logger:         logger,
		producer:       sprod,
		gasPriceFactor: gasPriceFactor,
	}
}

type manager struct {
	chainID        *big.Int
	sender         sender.Sender
	client         *ethclient.Client
	storage        storage.Storage
	logger         *zerolog.Logger
	producer       status.Producer
	nonceMutex     sync.Mutex
	gasPriceFactor *big.Rat
}

func (m *manager) SendTx(ctx context.Context, req *TransactionRequest) error {
	m.nonceMutex.Lock()
	defer m.nonceMutex.Unlock()

	head, err := m.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve head block: %w", err)
	}

	nonce := req.Nonce

	if nonce == nil {
		pNonce, err := m.client.PendingNonceAt(ctx, m.sender.Address())
		if err != nil {
			return fmt.Errorf("failed to retrieve nonce: %w", err)
		}

		nonce = &pNonce
	}

	signer := types.LatestSignerForChainID(m.chainID)

	gasPriceEst, err := m.client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve gas price estimate: %w", err)
	}

	gasPrice := new(big.Int).Div(new(big.Int).Mul(gasPriceEst, m.gasPriceFactor.Num()), m.gasPriceFactor.Denom())

	callMsg := ethereum.CallMsg{
		From:     m.sender.Address(),
		To:       &req.To,
		GasPrice: gasPrice,
		Data:     req.Data,
	}

	gasLimit, err := m.client.EstimateGas(ctx, callMsg)
	if err != nil {
		return fmt.Errorf("failed to estimate gas usage: %w", err)
	}

	txd := &types.LegacyTx{
		Nonce:    *nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       &req.To,
		Data:     req.Data,
	}

	tx := types.NewTx(txd)

	sigHash := signer.Hash(tx)
	sigBytes, err := m.sender.Sign(ctx, sigHash)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}
	signedTx, err := tx.WithSignature(signer, sigBytes)
	if err != nil {
		return fmt.Errorf("failed to attach signature to transaction: %w", err)
	}
	txHash := signedTx.Hash()

	if req.Nonce == nil {
		storeTx := &storage.Transaction{
			ID:   req.ID,
			To:   req.To,
			Data: req.Data,

			Nonce:    *nonce,
			GasPrice: gasPrice,
			Hash:     txHash,

			SubmittedBlock: &storage.Block{
				Number: head.Number,
				Hash:   head.Hash(),
			},
		}

		m.logger.Info().Str("id", req.ID).Interface("tx", storeTx).Msg("Sending transaction.")

		err = m.storage.New(storeTx)
		if err != nil {
			return fmt.Errorf("failed to store transaction: %w", err)
		}
	} else {
		err = m.storage.SetBoosted(req.ID, &storage.Block{Number: head.Number, Hash: head.Hash()}, gasPrice, txHash)
		if err != nil {
			return fmt.Errorf("failed to store boosted transaction: %w", err)
		}
	}

	err = m.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	if req.Nonce == nil {
		m.producer.Submitted(&status.SubmittedMsg{
			ID:   req.ID,
			Hash: txHash,
		})
	}

	return nil
}

func (m *manager) Receipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return m.client.TransactionReceipt(ctx, txHash)
}

func (m *manager) Head(ctx context.Context) (*types.Header, error) {
	return m.client.HeaderByNumber(ctx, nil)
}
