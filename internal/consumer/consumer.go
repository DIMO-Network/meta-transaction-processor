package consumer

import (
	"context"
	"encoding/json"

	"github.com/DIMO-Network/meta-transaction-processor/internal/manager"
	"github.com/DIMO-Network/shared"
	"github.com/Shopify/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

type consumer struct {
	logger  *zerolog.Logger
	client  *ethclient.Client
	manager manager.Manager
}

type TransactionEventData struct {
	ID   string `json:"id"`
	To   string `json:"to"`
	Data string `json:"data"`
}

func (c *consumer) Setup(sarama.ConsumerGroupSession) error { return nil }

func (c *consumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (c *consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		event := &shared.CloudEvent[TransactionEventData]{}

		err := json.Unmarshal(msg.Value, event)
		if err != nil {
			c.logger.Err(err).Int32("partition", msg.Partition).Int64("offset", msg.Offset).Msg("Couldn't parse message, skipping.")
			continue
		}

		c.logger.Info().Interface("requet", event.Data).Msg("Got transaction request.")

		to := common.HexToAddress(event.Data.To)
		data := common.FromHex(event.Data.Data)

		c.logger.Info().Interface("request", event.Data).Msg("Got meta-transaction request.")

		err = c.manager.SendTx(session.Context(), &manager.TransactionRequest{ID: event.Data.ID, To: to, Data: data})
		if err != nil {
			c.logger.Err(err).Msg("Error sending transaction.")
		}

		session.MarkMessage(msg, "")
	}
	return nil
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
