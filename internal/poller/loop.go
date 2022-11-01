package poller

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/DIMO-Network/meta-transaction-processor/internal/manager"
	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/meta-transaction-processor/internal/storage"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

type Poller struct {
	store              storage.Storage
	log                *zerolog.Logger
	eth                *ethclient.Client
	confirmationBlocks *big.Int
	man                manager.Manager
	prod               status.Producer
}

func NewPoller(store storage.Storage, log *zerolog.Logger, eth *ethclient.Client, confirmationBlocks *big.Int, man manager.Manager, prod status.Producer) *Poller {
	return &Poller{
		store:              store,
		log:                log,
		eth:                eth,
		confirmationBlocks: confirmationBlocks,
		man:                man,
		prod:               prod,
	}
}

func (p *Poller) Poll(ctx context.Context) error {
	head, err := p.eth.HeaderByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	pollBlock := head.Number

	txes, err := p.store.List()
	if err != nil {
		return err
	}

	for _, tx := range txes {
		err := p.CheckMined(ctx, pollBlock, tx)
		if err != nil {
			return err
		}
	}

	return nil
}

// CheckMined tries to observe whether transactions have been mined, and whether mined
// transactions have moved between blocks. Storage is updated with block information
// and an event is emitted the first time mining is observed.
func (p *Poller) CheckMined(ctx context.Context, pollBlock *big.Int, tx *storage.Transaction) (err error) {
	logger := p.log.With().Str("id", tx.ID).Str("hash", tx.Hash.Hex()).Logger()

	rec, err := p.man.Receipt(ctx, tx.Hash)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {

		}
	}

	if tx.MinedBlock == nil || new(big.Int).Sub(pollBlock, tx.MinedBlock.Number).Cmp(p.confirmationBlocks) >= 0 {
		rec, err := p.man.Receipt(ctx, tx.Hash)
		if err != nil {
			// We shouldn't expect this right away.
			if errors.Is(err, ethereum.NotFound) {
				if tx.MinedBlock != nil {
					logger.Warn().Msg("Transaction un-mined.")
				}
				return nil
			}
			return err
		}

		minedBlockStore := &storage.Block{
			Number: rec.BlockNumber,
			Hash:   rec.BlockHash,
		}

		if tx.MinedBlock == nil || tx.MinedBlock.Hash != minedBlockStore.Hash {
			if tx.MinedBlock == nil {
				logger.Info().Msg("Transaction mined.")
				p.prod.Mined(&status.MinedMsg{ID: tx.ID, Hash: tx.Hash})
			} else {
				logger.Info().Msg("Transaction moved between blocks.")
			}

			err = p.store.SetTxMined(tx.ID, minedBlockStore)
			if err != nil {
				return err
			}
		} else {
			logger.Info().Bool("successful", rec.Status == 1).Msg("Transaction confirmed.")

			logs := []*status.Log{}
			for _, log := range rec.Logs {
				logs = append(logs, &status.Log{Address: log.Address, Topics: log.Topics, Data: log.Data})
			}

			p.prod.Confirmed(
				&status.ConfirmedMsg{
					ID:         tx.ID,
					Hash:       tx.Hash,
					Successful: rec.Status == 1,
					Logs:       logs,
				},
			)

			p.store.Remove(tx.ID)
			if err != nil {
				logger.Err(err).Msg("Failed to remove transaction from store.")
			}
		}
	}

	return nil
}

type txIteration struct {
	pollBlock *big.Int
	man       manager.Manager
	txes      []*storage.Transaction
	index     int
	boosting  bool
	gasPrice  *big.Int
	eth       *ethclient.Client
}

var mineWait = big.NewInt(5)

func (i *txIteration) Next(ctx context.Context) error {
	tx := i.txes[i.index]

	if !i.boosting {
		rec, err := i.eth.TransactionReceipt(ctx, tx.Hash)
		if err != nil {
			if errors.Is(err, ethereum.NotFound) {
				if new(big.Int).Sub(i.pollBlock, tx.CreationBlock.Number).Cmp(mineWait) >= 0 {
					gasPrice, err := i.eth.SuggestGasPrice(ctx)
					if err != nil {
						return err
					}

					i.gasPrice = gasPrice
				}
			} else {
				return err
			}
		}

	} else if i.boosting && tx.GasPrice.Cmp(i.gasPrice) < 0 {
		req := manager.TransactionRequest{
			ID:       tx.ID,
			To:       tx.To,
			Data:     tx.Data,
			GasPrice: i.gasPrice,
			Nonce:    tx.Nonce,
		}
		err := i.man.SendTx(ctx, &req)
		if err != nil {
			return err
		}
	}

	i.index++

	return nil
}
