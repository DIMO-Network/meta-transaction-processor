package consumer

import (
	"context"
	"encoding/json"

	"github.com/DIMO-Network/meta-transaction-processor/internal/models"
	"github.com/DIMO-Network/shared"
	"github.com/DIMO-Network/shared/db"
	"github.com/Shopify/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

var requestsTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Namespace: "meta_transaction_processor",
		Subsystem: "consumer",
		Name:      "requests_total",
	},
)

type consumer struct {
	logger *zerolog.Logger
	dbs    db.Store
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
				session.MarkMessage(msg, "")
				continue
			}

			data := event.Data

			logger = logger.With().Str("requestId", data.ID).Str("contract", data.To.Hex()).Logger()

			logger.Info().Msg("Got transaction request.")

			tx := models.MetaTransactionRequest{
				ID:   data.ID,
				To:   data.To.Bytes(),
				Data: data.Data,
			}

			// Don't really want to update.
			if err := tx.Upsert(session.Context(), c.dbs.DBS().Writer, false, []string{models.MetaTransactionRequestColumns.ID}, boil.None(), boil.Infer()); err != nil {
				logger.Err(err).Msg("Error saving transaction.")
				return err
			}

			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}

func New(ctx context.Context, name string, topic string, kafkaClient sarama.Client, logger *zerolog.Logger, dbs db.Store) error {
	group, err := sarama.NewConsumerGroupFromClient(name, kafkaClient)
	if err != nil {
		return err
	}

	consumer := &consumer{logger: logger, dbs: dbs}

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
