package main

import (
	"context"
	"math/big"
	"os"
	"os/signal"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/consumer"
	"github.com/DIMO-Network/meta-transaction-processor/manager"
	"github.com/DIMO-Network/meta-transaction-processor/status"
	"github.com/DIMO-Network/meta-transaction-processor/storage"
	"github.com/Shopify/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

const RPCURL = "http://127.0.0.1:8545"

type Signer interface {
}

var confirmationBlocks = big.NewInt(12)

type EmitLog struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
}

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("app", "meta-transaction-processor").Logger()
	ctx, cancel := context.WithCancel(context.Background())

	senderPrivateKey := os.Getenv("SENDER_PRIVATE_KEY")

	sender, err := manager.KeyAccount(senderPrivateKey)
	if err != nil {
		logger.Fatal().Err(err).Msg("Couldn't load private key for sender.")
	}

	ethClient, err := ethclient.Dial(RPCURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Ethereum client.")
	}

	store := storage.NewMemStorage()

	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Producer.Return.Successes = true

	kafkaClient, err := sarama.NewClient([]string{"localhost:9092"}, kafkaConfig)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Kafka client.")
	}

	sprod, err := status.NewKafka(ctx, "topic.transaction.request.status", kafkaClient)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Kafka transaction status producer.")
	}

	manager := manager.New(ethClient, big.NewInt(31337), sender, store, &logger, sprod)

	go func() {
		consumer.New(ctx, "meta-transaction-processor", "topic.transaction.request.send", kafkaClient, &logger, ethClient, manager)
	}()

	logger.Info().Msg("Started.")

	go func() {
		for range time.NewTicker(10 * time.Second).C {
			head, err := ethClient.HeaderByNumber(ctx, nil)
			if err != nil {
				logger.Err(err).Msg("Failed to get block.")
				continue
			}

			headNumber := head.Number

			txes, _ := store.List()
			for _, tx := range txes {
				logger := logger.With().Str("id", tx.ID).Str("hash", tx.Hash.Hex()).Logger()
				if tx.MinedBlock == nil || new(big.Int).Sub(headNumber, tx.MinedBlock.Number).Cmp(confirmationBlocks) >= 0 {
					rec, err := manager.Receipt(ctx, tx.Hash)
					if err != nil {
						logger.Err(err).Msg("Failed to get receipt.")
						continue
					}

					minedBlock, err := ethClient.BlockByHash(ctx, rec.BlockHash)
					if err != nil {
						continue
					}
					minedBlockStore := &storage.Block{
						Number:    minedBlock.Number(),
						Hash:      minedBlock.Hash(),
						Timestamp: minedBlock.Time(),
					}
					if tx.MinedBlock == nil {
						logger.Info().Msg("Transaction mined.")
						store.SetTxMined(tx.ID, minedBlockStore)
						sprod.Mined(&status.MinedMsg{ID: tx.ID, Hash: tx.Hash, Block: minedBlockStore})
					} else {
						logger.Info().Msg("Transaction confirmed.")
						for _, l := range rec.Logs {
							logger.Info().Interface("log", l).Msg("Logged.")
						}
						logs := []*status.Log{}
						for _, log := range rec.Logs {
							logs = append(logs, &status.Log{Address: log.Address, Topics: log.Topics, Data: log.Data})
						}
						logger.Info().Interface("logs", rec.Logs).Msg("Don Logs.")
						sprod.Confirmed(&status.ConfirmedMsg{ID: tx.ID, Hash: tx.Hash, Block: minedBlockStore, Successful: rec.Status == 1, Logs: logs})
						err := store.Remove(tx.ID)
						if err != nil {
							logger.Err(err).Msg("Failed to remove transaction from store.")
							continue
						}
					}
				}

			}
		}
	}()

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, os.Interrupt)

	sig := <-sigterm
	logger.Info().Str("signal", sig.String()).Msg("Received signal, terminating.")

	cancel()
}
