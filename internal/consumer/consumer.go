package consumer

import (
	"context"
	"encoding/json"

	"github.com/DIMO-Network/meta-transaction-processor/internal/manager"
	"github.com/DIMO-Network/shared"
	"github.com/Shopify/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

var requestsTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Namespace: "meta_transaction_processor",
		Subsystem: "consumer",
		Name:      "requests_total",
	},
)

type consumer struct {
	logger  *zerolog.Logger
	client  *ethclient.Client
	manager manager.Manager
}

type TransactionEventData struct {
	ID   string         `json:"id"`
	To   common.Address `json:"to"`
	Data hexutil.Bytes  `json:"data"`
}

func (c *consumer) Setup(sarama.ConsumerGroupSession) error { return nil }

func (c *consumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (c *consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg := <-claim.Messages():
			requestsTotal.Inc()
			logger := c.logger.With().Int32("partition", msg.Partition).Int64("offset", msg.Offset).Logger()

			var event shared.CloudEvent[TransactionEventData]
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				logger.Err(err).Msg("Couldn't parse request, skipping.")
				continue
			}

			data := event.Data

			logger.Info().Str("requestId", data.ID).Str("toAddress", hexutil.Encode(data.To.Bytes())).Msg("Got transaction request.")

			req := &manager.TransactionRequest{ID: data.ID, To: data.To, Data: data.Data}

			err := c.manager.SendTx(session.Context(), req)
			if err != nil {
				logger.Err(err).Msg("Error sending transaction.")
			}

			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

func New(ctx context.Context, name string, topic string, kafkaClient sarama.Client, logger *zerolog.Logger, ethClient *ethclient.Client, manager manager.Manager) error {
	group, err := sarama.NewConsumerGroupFromClient(name, kafkaClient)
	if err != nil {
		return err
	}

	consumer := &consumer{logger: logger, client: ethClient, manager: manager}

	for {
		err := group.Consume(ctx, []string{topic}, consumer)
		if err != nil {
			logger.Err(err).Msg("Consumer group session did not terminate gracefully.")
		}
		if ctx.Err() != nil {
			// Context canceled, so quit.
			return nil
		}
	}
}
