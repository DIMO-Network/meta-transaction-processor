package main

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/internal/config"
	"github.com/DIMO-Network/meta-transaction-processor/internal/consumer"
	appmetrics "github.com/DIMO-Network/meta-transaction-processor/internal/metrics"
	"github.com/DIMO-Network/meta-transaction-processor/internal/rpc"
	"github.com/DIMO-Network/meta-transaction-processor/internal/sender"
	"github.com/DIMO-Network/meta-transaction-processor/internal/status"
	"github.com/DIMO-Network/meta-transaction-processor/internal/ticker"
	mtpgrpc "github.com/DIMO-Network/meta-transaction-processor/pkg/grpc"
	"github.com/DIMO-Network/shared"
	"github.com/DIMO-Network/shared/db"
	"github.com/DIMO-Network/shared/middleware/metrics"
	"github.com/IBM/sarama"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/burdiyan/kafkautil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type EmitLog struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	logger := zerolog.New(os.Stdout).With().Timestamp().Str("app", "meta-transaction-processor").Logger()
	settings, err := shared.LoadConfig[config.Settings]("settings.yaml")
	if err != nil {
		logger.Fatal().Err(err).Msg("Couldn't load settings.")
	}

	logger.Info().Msgf("Loaded settings: %d second block time, %d blocks for confirmations, %d blocks before boosting.", settings.BlockTime, settings.ConfirmationBlocks, settings.BoostAfterBlocks)

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "print-kms-address":
			if len(os.Args) < 3 {
				logger.Fatal().Msg("Key id required.")
			}

			keyID := os.Args[2]
			kmsc, err := makeKMSClient(ctx, &settings)
			if err != nil {
				logger.Fatal().Err(err).Msg("Failed to load AWS config.")
			}

			send, err := sender.FromKMS(ctx, kmsc, keyID)
			if err != nil {
				logger.Fatal().Err(err).Msg("Failed to construct account.")
			}

			logger.Info().Msgf("Key corresponds to address %s.", send.Address())
			return
		case "migrate":
			command := "up"
			if len(os.Args) > 2 {
				command = os.Args[2]
				if command == "down-to" || command == "up-to" {
					command = command + " " + os.Args[3]
				}
			}
			migrateDatabase(logger, &settings, command)
			return
		default:
			logger.Error().Msgf("Unrecognized sub-command %q.", os.Args[1])
		}
	}

	pdb := db.NewDbConnectionFromSettings(ctx, &settings.DB, true)
	pdb.WaitForDB(logger)

	confirmationBlocks := big.NewInt(settings.ConfirmationBlocks)

	boostAfterBlocks := big.NewInt(settings.BoostAfterBlocks)

	senders, err := createSenders(ctx, &settings, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create sender.")
	}

	ethClient, err := ethclient.Dial(settings.EthereumRPCURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Ethereum client.")
	}

	kafkaClient, err := createKafka(&settings)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Kafka client.")
	}

	sprod, err := status.NewKafka(ctx, settings.TransactionStatusTopic, kafkaClient, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create Kafka transaction status producer.")
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("Couldn't retrieve chain id.")
	}

	logger.Info().Msgf("Chain id is %d.", chainID)

	go func() {
		err := consumer.New(ctx, "meta-transaction-processor", settings.TransactionRequestTopic, kafkaClient, &logger, pdb, len(senders))
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to create Kafka consumer.")
		}
	}()

	var tickerGroup sync.WaitGroup

	for i, sender := range senders {
		watcher := ticker.New(&logger, sprod, confirmationBlocks, boostAfterBlocks, pdb, ethClient, chainID, sender, i, settings.DisableBoosting)

		tickerGroup.Add(1)

		go func() {
			defer tickerGroup.Done()
			ticker := time.NewTicker(time.Duration(settings.BlockTime) * time.Second)
			for {
				select {
				case <-ticker.C:
					appmetrics.TicksTotal.Inc()
					if err := watcher.Tick(ctx); err != nil {
						appmetrics.TickErrorsTotal.With(prometheus.Labels{"walletIndex": strconv.Itoa(i)}).Inc()
						log.Err(err).Msg("Error during tick.")
					}
				case <-ctx.Done():
					ticker.Stop()
					return
				}
			}
		}()
	}

	go startGRPCServer(&settings, &logger, pdb)

	monApp := serveMonitoring(settings.MonitoringPort, &logger)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, os.Interrupt, syscall.SIGTERM)

	sig := <-sigterm
	logger.Info().Str("signal", sig.String()).Msg("Received signal, terminating.")

	cancel()
	err = monApp.Shutdown()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to shutdown monitoring web server.")
	}
	tickerGroup.Wait()
}

func createSenders(ctx context.Context, settings *config.Settings, logger *zerolog.Logger) ([]sender.Sender, error) {
	if settings.PrivateKeyMode {
		logger.Warn().Msg("Using injected private keys. Never do this in production.")

		rawPKs := strings.Split(settings.SenderPrivateKeys, ",")
		senders := make([]sender.Sender, len(rawPKs))

		for i, pk := range rawPKs {
			send, err := sender.FromKey(pk)
			if err != nil {
				return nil, err
			}
			logger.Info().Str("address", send.Address().Hex()).Msg("Loaded private key account.")
			senders[i] = send
		}

		return senders, nil
	} else {
		kmsc, err := makeKMSClient(ctx, settings)
		if err != nil {
			return nil, err
		}

		keyIDs := strings.Split(settings.KMSKeyIDs, ",")
		senders := make([]sender.Sender, len(keyIDs))

		for i, keyID := range keyIDs {
			send, err := sender.FromKMS(ctx, kmsc, keyID)
			if err != nil {
				return nil, err
			}
			senders[i] = send
			logger.Info().Msgf("Loaded KMS key %s, address %s, into slot %d.", keyID, send.Address().Hex(), i)
		}

		return senders, nil
	}
}

func makeKMSClient(ctx context.Context, settings *config.Settings) (*kms.Client, error) {
	conf, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(settings.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	kmsc := kms.NewFromConfig(conf, func(o *kms.Options) {
		if settings.AWSEndpoint != "" {
			o.BaseEndpoint = &settings.AWSEndpoint
		}
	})
	return kmsc, nil
}

func createKafka(settings *config.Settings) (sarama.Client, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V2_8_1_0                                    // Version from production.
	kafkaConfig.Producer.Partitioner = kafkautil.NewJVMCompatiblePartitioner // Use the murmur2 hash from the official client.
	kafkaConfig.Producer.Return.Successes = true                             // Synchronous producer.

	return sarama.NewClient(strings.Split(settings.KafkaServers, ","), kafkaConfig)
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

	logger.Info().Msgf("Started monitoring web server on port %s.", port)

	return monApp
}

func startGRPCServer(settings *config.Settings, logger *zerolog.Logger, dbs db.Store) {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%s", settings.GRPCPort))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to listen for grpc server.")
	}
	gp := rpc.GRPCPanicker{Logger: logger}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			metrics.GRPCMetricsAndLogMiddleware(logger),
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_prometheus.UnaryServerInterceptor,
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(gp.GRPCPanicRecoveryHandler)),
		)),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	)

	mtpgrpc.RegisterMetaTransactionServiceServer(server, rpc.NewMetaTransactionService(settings, logger, dbs))

	if err := server.Serve(listen); err != nil {
		logger.Fatal().Err(err).Msg("gRPC server terminated unexpectedly")
	}
}
