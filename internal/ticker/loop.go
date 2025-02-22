package ticker

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"strconv"

	"github.com/DIMO-Network/meta-transaction-processor/internal/models"
	"github.com/DIMO-Network/meta-transaction-processor/internal/sender"
	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/shared/db"
	"github.com/ericlagergren/decimal"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
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
	HeaderByNumber(ctx context.Context, number *big.Int) (*ethtypes.Header, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*ethtypes.Receipt, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error)
	SendTransaction(ctx context.Context, tx *ethtypes.Transaction) error
	PendingNonceAt(ctx context.Context, account common.Address) (uint64, error)
}

// ethJSONRPCError mirrors the methods of the unexported rpc.jsonError type from
// go-ethereum.
type ethJSONRPCError interface {
	rpc.Error
	rpc.DataError
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
	disableBoosting    bool
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
	disableBoosting bool,
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
		disableBoosting:    disableBoosting,
	}
}

var cols = models.MetaTransactionRequestColumns

var latestBlock = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "meta_transaction_processor",
	Name:      "latest_block",
})

var submittedTxBlockAge = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "meta_transaction_processor",
		Name:      "submitted_tx_block_age",
	},
	[]string{"wallet"},
)

func (w *Watcher) Tick(ctx context.Context) error {
	head, err := w.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve latest block: %w", err)
	}

	headNum := head.Number
	headNumFloat, _ := headNum.Float64()

	latestBlock.Set(headNumFloat)

	logger := w.logger.With().Int64("block", headNum.Int64()).Int("walletIndex", w.walletIndex).Logger()

	// There's at most one submitted transaction per wallet.
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
		subBlockNum, _ := activeTx.SubmittedBlockNumber.Float64()

		// TODO(elffjs): Can we do this label-setting once?
		submittedTxBlockAge.With(prometheus.Labels{"wallet": strconv.Itoa(w.walletIndex)}).Set(headNumFloat - subBlockNum)

		logger := logger.With().Str("requestId", activeTx.ID).Str("contract", common.BytesToAddress(activeTx.To).Hex()).Logger()

		rec, err := w.client.TransactionReceipt(ctx, common.BytesToHash(activeTx.Hash.Bytes))
		if err != nil {
			if err != ethereum.NotFound {
				return fmt.Errorf("error retrieving transaction receipt: %w", err)
			}
			// Transaction not included yet.

			if !activeTx.MinedBlockNumber.IsZero() {
				logger.Info().Msg("Transaction no longer in the canonical chain.")
				activeTx.MinedBlockNumber = types.NewNullDecimal(nil)
				activeTx.MinedBlockHash = null.Bytes{}
				_, err := activeTx.Update(ctx, w.dbs.DBS().Writer, boil.Whitelist(cols.MinedBlockNumber, cols.MinedBlockHash, cols.UpdatedAt))
				if err != nil {
					return err
				}
			}

			lastSend := activeTx.SubmittedBlockNumber.Int(nil)
			if !activeTx.BoostedBlockNumber.IsZero() {
				lastSend = activeTx.BoostedBlockNumber.Int(nil)
			}

			if new(big.Int).Sub(headNum, lastSend).Cmp(w.boostAfterBlocks) >= 0 {
				if w.disableBoosting {
					logger.Warn().Msgf("Would have boosted after %d blocks, but boosting disabled.", new(big.Int).Sub(headNum, lastSend))
					return nil
				}
				signer := ethtypes.LatestSignerForChainID(w.chainID)

				gasPrice, err := w.client.SuggestGasPrice(ctx)
				if err != nil {
					return fmt.Errorf("failed to retrieve gas price estimate: %w", err)
				}

				gasPrice = new(big.Int).Mul(common.Big2, gasPrice)

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
					logger.Err(err).Msg("Failed to estimate gas usage for transaction.")

					var outData []byte

					// TODO(elffjs): More logging if this doesn't meet our expectations.
					// There is no contract around these error values.
					if jerr, ok := err.(ethJSONRPCError); ok {
						logger.Error().Str("message", jerr.Error()).Int("code", jerr.ErrorCode()).Interface("data", jerr.ErrorData()).Msg("Transaction failed with a JSON-RPC error.")
						if hexData, ok := jerr.ErrorData().(string); ok {
							if data, err := hexutil.Decode(hexData); err == nil && len(data) != 0 {
								outData = data
							}
						}
					} else {
						return fmt.Errorf("error estimating gas: %w", err)
					}

					w.prod.Failed(&status.FailedMsg{ID: activeTx.ID, Data: outData})

					_, err := activeTx.Delete(ctx, w.dbs.DBS().Writer)
					if err != nil {
						return fmt.Errorf("failed to delete un-estimateable transaction: %w", err)
					}

					return nil
				}

				gasLimit = 2 * gasLimit

				nonce, _ := activeTx.Nonce.Uint64()

				txd := &ethtypes.LegacyTx{
					Nonce:    nonce,
					GasPrice: gasPrice,
					Gas:      gasLimit,
					To:       callMsg.To,
					Data:     callMsg.Data,
				}

				tx := ethtypes.NewTx(txd)

				sigHash := signer.Hash(tx)
				sigBytes, err := w.sender.Sign(ctx, sigHash)
				if err != nil {
					return fmt.Errorf("failed to sign transaction: %w", err)
				}

				signedTx, err := tx.WithSignature(signer, sigBytes)
				if err != nil {
					return fmt.Errorf("failed to attach signature to transaction: %w", err)
				}

				activeTx.BoostedBlockNumber = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(headNum, 0))
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

		// Transaction included.
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

		conf := new(big.Int).Sub(headNum, rec.BlockNumber)

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

	// At this point, there's nothing in the table that's been submitted. Try to submit something.
	submittedTxBlockAge.With(prometheus.Labels{"wallet": strconv.Itoa(w.walletIndex)}).Set(0)

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

	signer := ethtypes.LatestSignerForChainID(w.chainID)

	// TODO(elffjs): Look at EIP-1559 stuff. Seems weird on Polygon and their oracles
	// are weird.
	gasPrice, err := w.client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve gas price estimate: %w", err)
	}

	gasPrice = new(big.Int).Mul(common.Big2, gasPrice)

	callMsg := ethereum.CallMsg{
		From:     w.sender.Address(),
		To:       Ref(common.BytesToAddress(sendTx.To)),
		GasPrice: gasPrice,
		Data:     sendTx.Data,
	}

	gasLimit, err := w.client.EstimateGas(ctx, callMsg)
	if err != nil {
		logger.Err(err).Msg("Failed to estimate gas usage for transaction.")

		var outData []byte

		// TODO(elffjs): More logging if this doesn't meet our expectations.
		// There is no contract around these error values.
		if jerr, ok := err.(ethJSONRPCError); ok {
			logger.Error().Str("message", jerr.Error()).Int("code", jerr.ErrorCode()).Interface("data", jerr.ErrorData()).Msg("Transaction failed with a JSON-RPC error.")
			if hexData, ok := jerr.ErrorData().(string); ok {
				if data, err := hexutil.Decode(hexData); err == nil && len(data) != 0 {
					outData = data
				}
			}
		} else {
			return fmt.Errorf("error estimating gas: %w", err)
		}

		w.prod.Failed(&status.FailedMsg{ID: sendTx.ID, Data: outData})

		_, err := sendTx.Delete(ctx, w.dbs.DBS().Writer)
		if err != nil {
			return fmt.Errorf("failed to delete un-estimateable transaction: %w", err)
		}

		return nil
	}

	gasLimit = 2 * gasLimit

	txd := &ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gasLimit,
		To:       callMsg.To,
		Data:     callMsg.Data,
	}

	tx := ethtypes.NewTx(txd)

	sigHash := signer.Hash(tx)
	sigBytes, err := w.sender.Sign(ctx, sigHash)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	signedTx, err := tx.WithSignature(signer, sigBytes)
	if err != nil {
		return fmt.Errorf("failed to attach signature to transaction: %w", err)
	}

	logger.Info().Msgf("Submitting transaction with nonce %d and hash %s.", nonce, signedTx.Hash())

	err = w.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	sendTx.SubmittedBlockNumber = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(headNum, 0))
	sendTx.SubmittedBlockHash = null.BytesFrom(head.Hash().Bytes())
	sendTx.Nonce = types.NewNullDecimal(new(decimal.Big).SetUint64(nonce))
	sendTx.GasPrice = types.NewNullDecimal(new(decimal.Big).SetBigMantScale(gasPrice, 0))
	sendTx.Hash = null.BytesFrom(signedTx.Hash().Bytes())

	_, err = sendTx.Update(ctx, w.dbs.DBS().Writer, boil.Whitelist(
		cols.SubmittedBlockHash,
		cols.Hash,
		cols.SubmittedBlockNumber,
		cols.Nonce,
		cols.GasPrice,
		cols.UpdatedAt,
	))
	if err != nil {
		return err
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
