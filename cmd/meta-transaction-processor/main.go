package main

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strings"
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
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"github.com/burdiyan/kafkautil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type EmitLog struct {
	Address common.Address
	Topics  []common.Hash
	Data    []byte
}

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("app", "meta-transaction-processor").Logger()
	settings, err := shared.LoadConfig[config.Settings]("settings.yaml")
	if err != nil {
		logger.Fatal().Err(err).Msg("Couldn't load settings.")
	}

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		command := "up"
		if len(os.Args) > 2 {
			command = os.Args[2]
			if command == "down-to" || command == "up-to" {
				command = command + " " + os.Args[3]
			}
		}
		migrateDatabase(logger, &settings, command)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	pdb := db.NewDbConnectionFromSettings(ctx, &settings.DB, true)
	pdb.WaitForDB(logger)

	logger.Info().Msgf("Loaded settings: %d second block time, %d blocks for confirmations, %d blocks before boosting.", settings.BlockTime, settings.ConfirmationBlocks, settings.BoostAfterBlocks)

	confirmationBlocks := big.NewInt(settings.ConfirmationBlocks)

	boostAfterBlocks := big.NewInt(settings.BoostAfterBlocks)

	send, err := createSender(ctx, &settings, &logger)
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

	go func() {
		err := consumer.New(ctx, "meta-transaction-processor", settings.TransactionRequestTopic, kafkaClient, &logger, pdb)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to create Kafka consumer.")
		}
	}()

	tickerDone := make(chan struct{})

	watcher := ticker.New(&logger, sprod, confirmationBlocks, boostAfterBlocks, pdb, ethClient, chainID, send)

	go func() {
		defer close(tickerDone)
		ticker := time.NewTicker(time.Duration(settings.BlockTime) * time.Second)
		for {
			select {
			case <-ticker.C:
				appmetrics.TicksTotal.Inc()
				if err := watcher.Tick(ctx); err != nil {
					appmetrics.TickErrorsTotal.Inc()
					log.Err(err).Msg("Error during tick.")
				}
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

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
	<-tickerDone
}

func createSender(ctx context.Context, settings *config.Settings, logger *zerolog.Logger) (sender.Sender, error) {
	if settings.PrivateKeyMode {
		logger.Warn().Msg("Using injected private key. Never do this in production.")
		send, err := sender.FromKey(settings.SenderPrivateKey)
		if err != nil {
			return nil, err
		}
		logger.Info().Str("address", send.Address().Hex()).Msg("Loaded private key account.")
		return send, nil
	} else {
		awsconf, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, err
		}
		kmsc := kms.NewFromConfig(awsconf)
		send, err := sender.FromKMS(ctx, kmsc, settings.KMSKeyID)
		if err != nil {
			return nil, err
		}
		logger.Info().Msgf("Loaded KMS key %s, address %s.", settings.KMSKeyID, send.Address().Hex())
		return send, nil
	}
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

	logger.Info().Msgf("Started monitoring web server on :%s.", port)

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
