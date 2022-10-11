package ticker

import (
	"context"
	"errors"
	"math/big"

	"github.com/DIMO-Network/meta-transaction-processor/internal/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

type ticker struct {
	store  storage.Storage
	logger *zerolog.Logger
	eth    ethclient.Client
}

func (t *ticker) Loop() {
	txes, err := t.store.List()
	if err != nil {
		return
	}

	if len(txes) == 0 {
		return
	}

	var currentBlock *big.Int

	t.logger.Debug().Int("count", len(txes)).Msg("Checking on stored transactions.")

	for _, tx := range txes {
		logger := t.logger.With().Str("requestId", tx.ID).Logger()

		if tx.MinedBlock != nil && new(big.Int).Sub(tx.MinedBlock.Number, currentBlock).Cmp(big.NewInt(6)) < 0 {
			continue
		}

		rcpt, err := t.eth.TransactionReceipt(context.TODO(), tx.Hash)
		if err != nil {
			if errors.Is(err, ethereum.NotFound) {
				logger.Warn().Msg("No receipt found.")
			} else {
				logger.Err(err).Msg("Error retrieving receipt.")
			}
			continue
		}

		if tx.MinedBlock == nil {
			t.store.SetTxMined(tx.ID, receiptBlock(rcpt))
		} else if tx.MinedBlock.Hash != rcpt.TxHash {
			logger.Warn().Msg("Transaction moved between blocks.")
			t.store.SetTxMined(tx.ID, receiptBlock(rcpt))
		}

		logger.Info().Msg("Transaction mined.")
	}
}

func receiptBlock(rcpt *types.Receipt) *storage.Block {
	return &storage.Block{Number: rcpt.BlockNumber, Hash: rcpt.TxHash}
}
