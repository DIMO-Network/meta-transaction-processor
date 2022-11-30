package ticker

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/internal/manager"
	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/meta-transaction-processor/internal/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/rs/zerolog"
)

var loopDurationSeconds = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Namespace: "meta_transaction_processor",
		Subsystem: "ticker",
		Name:      "iteration_duration_seconds",
	},
)

type Watcher struct {
	logger             *zerolog.Logger
	store              storage.Storage
	confirmationBlocks *big.Int
	manager            manager.Manager
	prod               status.Producer
}

func New(logger *zerolog.Logger, store storage.Storage, manager manager.Manager, prod status.Producer, confirmationBlocks *big.Int) *Watcher {
	return &Watcher{
		logger:             logger,
		store:              store,
		confirmationBlocks: confirmationBlocks,
		manager:            manager,
		prod:               prod,
	}
}

func (w *Watcher) Tick(ctx context.Context) error {
	startTime := time.Now()
	defer func() { loopDurationSeconds.Observe(float64(time.Since(startTime).Seconds())) }()

	txes, err := w.store.List()
	if err != nil {
		return fmt.Errorf("failed to retrieve monitored transactions: %w", err)
	}

	head, err := w.manager.Head(ctx)
	if err != nil {
		return err
	}

	for _, tx := range txes {
		logger := w.logger.With().Str("id", tx.ID).Str("hash", tx.Hash.String()).Logger()

		rec, err := w.manager.Receipt(ctx, tx.Hash)
		if err != nil {
			if errors.Is(err, ethereum.NotFound) {
				lastSubmit := tx.SubmittedBlock
				if tx.BoostedBlock != nil {
					lastSubmit = tx.BoostedBlock
				}

				if new(big.Int).Sub(head.Number, lastSubmit.Number).Cmp(w.confirmationBlocks) >= 0 {
					logger.Info().Msg("Boosting transaction.")
					w.manager.SendTx(ctx, &manager.TransactionRequest{ID: tx.ID, To: tx.To, Data: tx.Data, Nonce: &tx.Nonce})
				}
			}

			logger.Err(err).Msg("Failed to get receipt.")
			continue
		}

		minedBlock := &storage.Block{
			Number: rec.BlockNumber,
			Hash:   rec.BlockHash,
		}

		if tx.MinedBlock == nil {
			// Newly mined.
			logger.Info().Msg("Transaction mined.")
			err = w.store.SetMined(tx.ID, minedBlock)
			if err != nil {
				return err
			}
			w.prod.Mined(&status.MinedMsg{ID: tx.ID, Hash: tx.Hash})
		} else if new(big.Int).Sub(head.Number, tx.MinedBlock.Number).Cmp(w.confirmationBlocks) >= 0 {
			logger.Info().Msg("Transaction confirmed.")
			logs := make([]*status.Log, len(rec.Logs))

			for i, l := range rec.Logs {
				logs[i] = &status.Log{
					Address: l.Address,
					Topics:  l.Topics,
					Data:    l.Data,
				}
			}

			msg := &status.ConfirmedMsg{
				ID:         tx.ID,
				Hash:       tx.Hash,
				Successful: rec.Status == 1,
				Logs:       logs,
			}

			w.prod.Confirmed(msg)
			err := w.store.Remove(tx.ID)
			if err != nil {
				logger.Err(err).Msg("Failed to remove transaction from store.")
			}
		}
		// Otherwise, we're waiting for more confirmations.
	}

	return nil
}
