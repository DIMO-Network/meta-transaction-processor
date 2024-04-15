package ticker

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"

	"github.com/DIMO-Network/meta-transaction-processor/internal/models"
	"github.com/DIMO-Network/meta-transaction-processor/internal/sender"
	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/shared/db"
	"github.com/ericlagergren/decimal"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	eth_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/types"

	"github.com/rs/zerolog"
)

// EthClient contains all the ethclient.Client methods that we use.
type EthClient interface {
	BlockByNumber(ctx context.Context, number *big.Int) (*eth_types.Block, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*eth_types.Receipt, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
	SendTransaction(ctx context.Context, tx *eth_types.Transaction) error
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
}

type Watcher struct {
	logger             *zerolog.Logger
	confirmationBlocks *big.Int
	boostAfterBlocks   *big.Int
	prod               status.Producer
	dbs                db.Store
	client             EthClient
	sender             sender.Sender
	chainID            *big.Int
	walletIndex        int
}

func New(
	logger *zerolog.Logger,
	prod status.Producer,
	confirmationBlocks *big.Int,
	boostAfterBlocks *big.Int,
	dbs db.Store,
	client *ethclient.Client,
	chainID *big.Int,
	sender sender.Sender,
	walletIndex int,
) *Watcher {
	return &Watcher{
		logger:             logger,
		confirmationBlocks: confirmationBlocks,
		prod:               prod,
		boostAfterBlocks:   boostAfterBlocks,
		dbs:                dbs,
		client:             client,
		chainID:            chainID,
		sender:             sender,
		walletIndex:        walletIndex,
	}
}

var cols = models.MetaTransactionRequestColumns

var latestBlock = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "meta_transaction_processor",
	Name:      "latest_block",
})

var submittedTxBlockAge = promauto.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "meta_transaction_processor",
		Name:      "submitted_tx_block_age",
	},
)

func (w *Watcher) Tick(ctx context.Context) error {
	head, err := w.client.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve latest block: %w", err)
	}

	latestBlock.Set(float64(head.NumberU64()))

	logger := w.logger.With().Int64("block", head.Number().Int64()).Int("walletIndex", w.walletIndex).Logger()

	// There's at most one submitted transaction.
	if activeTx, err := models.MetaTransactionRequests(
		models.MetaTransactionRequestWhere.SubmittedBlockNumber.IsNotNull(),
		models.MetaTransactionRequestWhere.WalletIndex.EQ(w.walletIndex),
	).One(ctx, w.dbs.DBS().Reader); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
		// If there's no submitted transaction, fall through to trying to submit something new.
	} else {
		// We have a submitted but not confirmed (it would have been deleted) transaction.
		subBlockNum, _ := activeTx.SubmittedBlockNumber.Int64()
		submittedTxBlockAge.Set(float64(head.Number().Int64() - subBlockNum))

		logger := logger.With().Str("requestId", activeTx.ID).Str("contract", common.BytesToAddress(activeTx.To).Hex()).Logger()

		rec, err := w.client.TransactionReceipt(ctx, common.BytesToHash(activeTx.Hash.Bytes))
		if err != nil {
			if err != ethereum.NotFound {
				return err
			}

			// Might have been kicked out of the canonical chain.
			if !activeTx.MinedBlockNumber.IsZero() {
				logger.Info().Msg("Transaction no longer in the canonical chain.")
				activeTx.MinedBlockNumber = types.NewNullDecimal(nil)
				activeTx.MinedBlockHash = null.Bytes{}
				_, err := activeTx.Update(ctx, w.dbs.DBS().Writer, boil.Whitelist(cols.MinedBlockNumber, cols.MinedBlockHash))
				if err != nil {
					return err
				}
			}

			lastSend := activeTx.SubmittedBlockNumber.Int(nil)
			if !activeTx.BoostedBlockNumber.IsZero() {
				lastSend = activeTx.BoostedBlockNumber.Int(nil)
			}

			if new(big.Int).Sub(head.Number(), lastSend).Cmp(w.boostAfterBlocks) >= 0 {
				signer := eth_types.LatestSignerForChainID(w.chainID)

				gasPrice, err := w.client.SuggestGasPrice(ctx)
				if err != nil {
					return fmt.Errorf("failed to retrieve gas price estimate: %w", err)
				}

				gasPrice = new(big.Int).Mul(big.NewInt(2), gasPrice)

				oldGasPrice := activeTx.GasPrice.Int(nil)

				// Have to increase the old price by at least 10% to replace the transaction.
				newBar := new(big.Int).Mul(oldGasPrice, big.NewInt(120))
				newBar = new(big.Int).Div(newBar, big.NewInt(100))

				if newBar.Cmp(gasPrice) > 0 {
					gasPrice = newBar
				}

				callMsg := ethereum.CallMsg{
					From:     w.sender.Address(),
					To:       Ref(common.BytesToAddress(activeTx.To)),
					GasPrice: gasPrice,
					Data:     activeTx.Data,
				}

				gasLimit, err := w.client.EstimateGas(ctx, callMsg)
				if err != nil {
					return fmt.Errorf("failed to estimate gas usage: %w", err)
				}

				gasLimit = 2 * gasLimit

				nonce, _ := activeTx.Nonce.Uint64()

				txd := &eth_types.LegacyTx{
					Nonce:    nonce,
					GasPrice: gasPrice,
					Gas:      gasLimit,
					To:       callMsg.To,
					Data:     callMsg.Data,
				}

				tx := eth_types.NewTx(txd)

				sigHash := signer.Hash(tx)
				sigBytes, err := w.sender.Sign(ctx, sigHash)
				if err != nil {
					return fmt.Errorf("failed to sign transaction: %w", err)
				}

				signedTx, err := tx.WithSignature(signer, sigBytes)
				if err != nil {
					return fmt.Errorf("failed to attach signature to transaction: %w", err)
				}

				activeTx.BoostedBlockNumber = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(head.Number(), 0))
				activeTx.BoostedBlockHash = null.BytesFrom(signedTx.Hash().Bytes())
				activeTx.Nonce = types.NewNullDecimal(new(decimal.Big).SetUint64(nonce))
				activeTx.GasPrice = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(gasPrice, 0))
				activeTx.Hash = null.BytesFrom(signedTx.Hash().Bytes())

				_, err = activeTx.Update(ctx, w.dbs.DBS().Writer, boil.Whitelist(cols.BoostedBlockHash, cols.BoostedBlockNumber, cols.Nonce, cols.GasPrice, cols.UpdatedAt, cols.Hash))
				if err != nil {
					return err
				}

				logger.Info().Msgf("Boosting transaction with new gas price %d and hash %s.", gasPrice, signedTx.Hash())

				return w.client.SendTransaction(ctx, signedTx)
			} else {
				return nil
			}
		}

		if activeTx.MinedBlockNumber.IsZero() {
			logger.Info().Msgf("Transaction mined in block %d.", rec.BlockNumber)

			// We discount the possibility of sending mining and confirmation in the same tick.
			w.prod.Mined(&status.MinedMsg{ID: activeTx.ID, Hash: common.BytesToHash(activeTx.Hash.Bytes)})

			activeTx.MinedBlockNumber = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(rec.BlockNumber, 0))
			activeTx.MinedBlockHash = null.BytesFrom(rec.BlockHash.Bytes())

			_, err := activeTx.Update(ctx, w.dbs.DBS().Writer, boil.Whitelist(
				models.MetaTransactionRequestColumns.MinedBlockNumber,
				models.MetaTransactionRequestColumns.MinedBlockHash,
				models.MetaTransactionRequestColumns.UpdatedAt,
			))
			return err
		}

		conf := new(big.Int).Sub(head.Number(), rec.BlockNumber)

		if conf.Cmp(w.confirmationBlocks) >= 0 {
			logs := make([]*status.Log, len(rec.Logs))

			for i, l := range rec.Logs {
				logs[i] = &status.Log{
					Address: l.Address,
					Topics:  l.Topics,
					Data:    l.Data,
				}
			}

			msg := &status.ConfirmedMsg{
				ID:         activeTx.ID,
				Hash:       common.BytesToHash(activeTx.Hash.Bytes),
				Successful: rec.Status == 1,
				Logs:       logs,
			}

			logger.Info().Msg("Transaction confirmed.")

			w.prod.Confirmed(msg)

			_, err := activeTx.Delete(ctx, w.dbs.DBS().Writer)
			if err != nil {
				return err
			}
			// Fall through to maybe submitting something else.
		} else {
			if rec.BlockHash != common.BytesToHash(activeTx.MinedBlockHash.Bytes) {
				logger.Info().Msgf("Transaction moved from block %d to block %d.", activeTx.MinedBlockNumber.Int(nil), rec.BlockNumber)
				activeTx.MinedBlockNumber = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(rec.BlockNumber, 0))
				activeTx.MinedBlockHash = null.BytesFrom(rec.BlockHash.Bytes())

				_, err := activeTx.Update(ctx, w.dbs.DBS().Writer, boil.Whitelist(
					models.MetaTransactionRequestColumns.MinedBlockNumber,
					models.MetaTransactionRequestColumns.MinedBlockHash,
					models.MetaTransactionRequestColumns.UpdatedAt,
				))
				return err
			}
			// Otherwise, we're just waiting for more confirmations.
			return nil
		}
	}

	// At this point, there's nothing in the table that's been submitted.
	submittedTxBlockAge.Set(0)

	sendTx, err := models.MetaTransactionRequests(
		qm.OrderBy(models.MetaTransactionRequestColumns.ID+" ASC"),
		models.MetaTransactionRequestWhere.WalletIndex.EQ(w.walletIndex),
		qm.Limit(1),
	).One(ctx, w.dbs.DBS().Reader)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	logger = logger.With().Str("requestId", sendTx.ID).Str("contract", common.BytesToAddress(sendTx.To).Hex()).Logger()

	nonce, err := w.client.PendingNonceAt(ctx, w.sender.Address())
	if err != nil {
		return fmt.Errorf("failed to retrieve nonce: %w", err)
	}

	signer := eth_types.LatestSignerForChainID(w.chainID)

	gasPrice, err := w.client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve gas price estimate: %w", err)
	}

	gasPrice = new(big.Int).Mul(big.NewInt(2), gasPrice)

	callMsg := ethereum.CallMsg{
		From:     w.sender.Address(),
		To:       Ref(common.BytesToAddress(sendTx.To)),
		GasPrice: gasPrice,
		Data:     sendTx.Data,
	}

	gasLimit, err := w.client.EstimateGas(ctx, callMsg)
	if err != nil {
		return fmt.Errorf("failed to estimate gas usage: %w", err)
	}

	gasLimit = 2 * gasLimit

	txd := &eth_types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       callMsg.To,
		Data:     callMsg.Data,
	}

	tx := eth_types.NewTx(txd)

	sigHash := signer.Hash(tx)
	sigBytes, err := w.sender.Sign(ctx, sigHash)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	signedTx, err := tx.WithSignature(signer, sigBytes)
	if err != nil {
		return fmt.Errorf("failed to attach signature to transaction: %w", err)
	}

	sendTx.SubmittedBlockNumber = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(head.Number(), 0))
	sendTx.SubmittedBlockHash = null.BytesFrom(head.Hash().Bytes())
	sendTx.Nonce = types.NewNullDecimal(new(decimal.Big).SetUint64(nonce))
	sendTx.GasPrice = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(gasPrice, 0))
	sendTx.Hash = null.BytesFrom(signedTx.Hash().Bytes())

	_, err = sendTx.Update(ctx, w.dbs.DBS().Writer, boil.Whitelist(cols.SubmittedBlockHash, cols.Hash, cols.SubmittedBlockNumber, cols.Nonce, cols.GasPrice, cols.UpdatedAt))
	if err != nil {
		return err
	}

	logger.Info().Msgf("Submitting transaction with nonce %d and hash %s.", nonce, signedTx.Hash())

	err = w.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	w.prod.Submitted(&status.SubmittedMsg{
		ID:   sendTx.ID,
		Hash: signedTx.Hash(),
	})

	return nil
}

func Ref[A any](a A) *A {
	return &a
}
