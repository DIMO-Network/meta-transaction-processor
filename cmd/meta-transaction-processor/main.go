package main

import (
	"context"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/internal/config"
	"github.com/DIMO-Network/meta-transaction-processor/internal/consumer"
	"github.com/DIMO-Network/meta-transaction-processor/internal/manager"
	"github.com/DIMO-Network/meta-transaction-processor/internal/sender"
	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/meta-transaction-processor/internal/storage"
	"github.com/DIMO-Network/shared"
	"github.com/Shopify/sarama"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/burdiyan/kafkautil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

var confirmationBlocks = big.NewInt(12)

type EmitLog struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
}

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("app", "meta-transaction-processor").Logger()
	settings, err := shared.LoadConfig[config.Settings]("settings.yaml")
	if err != nil {
		logger.Fatal().Msg("Couldn't load settings.")
	}

	logger.Info().Int("chainId", settings.EthereumChainID).Msg("Loaded settings.")

	ctx, cancel := context.WithCancel(context.Background())

	var send sender.Sender

	if settings.PrivateKeyMode {
		logger.Warn().Msg("Using injected private key. Never do this in production.")
		send, err = sender.FromKey(settings.SenderPrivateKey)
		if err != nil {
			logger.Fatal().Err(err).Msg("Couldn't load private key for sender.")
		}
		logger.Info().Str("address", send.Address().Hex()).Msg("Loaded private key account.")
	} else {
		awsconf, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to load AWS configuration.")
		}
		kmsc := kms.NewFromConfig(awsconf)
		send, err = sender.FromKMS(ctx, kmsc, settings.KMSKeyID)
		if err != nil {
			logger.Fatal().Err(err).Msg("Couldn't create KMS signer.")
		}
		logger.Info().Str("address", send.Address().Hex()).Str("keyId", settings.KMSKeyID).Msg("Loaded KMS account.")
	}

	ethClient, err := ethclient.Dial(settings.EthereumRPCURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Ethereum client.")
	}

	store := storage.NewMemStorage()

	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V2_8_1_0
	kafkaConfig.Producer.Partitioner = kafkautil.NewJVMCompatiblePartitioner // Use murmur2 hash.
	kafkaConfig.Producer.Return.Successes = true

	kafkaClient, err := sarama.NewClient(strings.Split(settings.KafkaServers, ","), kafkaConfig)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Kafka client.")
	}

	sprod, err := status.NewKafka(ctx, settings.TransactionStatusTopic, kafkaClient, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Kafka transaction status producer.")
	}

	chainID := big.NewInt(int64(settings.EthereumChainID))

	manager := manager.New(ethClient, chainID, send, store, &logger, sprod)

	go func() {
		consumer.New(ctx, "meta-transaction-processor", settings.TransactionRequestTopic, kafkaClient, &logger, ethClient, manager)
	}()

	tickerDone := make(chan struct{})

	go func() {
		defer close(tickerDone)
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
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
							sprod.Confirmed(&status.ConfirmedMsg{ID: tx.ID, Hash: tx.Hash, Block: minedBlockStore, Successful: rec.Status == 1, Logs: logs})
							err := store.Remove(tx.ID)
							if err != nil {
								logger.Err(err).Msg("Failed to remove transaction from store.")
							}
						}
					}
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	monApp := serveMonitoring(settings.MonitoringPort, &logger)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, os.Interrupt)

	sig := <-sigterm
	logger.Info().Str("signal", sig.String()).Msg("Received signal, terminating.")

	cancel()
	monApp.Shutdown()
	<-tickerDone
}

func serveMonitoring(port string, logger *zerolog.Logger) *fiber.App {
	monApp := fiber.New(fiber.Config{DisableStartupMessage: true})

	// Health check.
	monApp.Get("/", func(c *fiber.Ctx) error { return nil })
	monApp.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	go func() {
		if err := monApp.Listen(":" + port); err != nil {
			logger.Fatal().Err(err).Str("port", port).Msg("Failed to start monitoring web server.")
		}
	}()

	logger.Info().Str("port", port).Msg("Started monitoring web server.")

	return monApp
}
