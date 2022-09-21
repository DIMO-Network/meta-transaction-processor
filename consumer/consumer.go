package consumer

import (
	"context"
	"encoding/json"

	"github.com/DIMO-Network/meta-transaction-processor/manager"
	"github.com/DIMO-Network/shared"
	"github.com/Shopify/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
)

type Consumer struct {
	Logger    *zerolog.Logger
	EthClient *ethclient.Client
	Manager   manager.Manager
}

type TransactionEventData struct {
	ID   string `json:"id"`
	To   string `json:"to"`
	Data string `json:"data"`
}

func (c *Consumer) Setup(sarama.ConsumerGroupSession) error { return nil }

func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		event := &shared.CloudEvent[TransactionEventData]{}

		err := json.Unmarshal(msg.Value, event)
		if err != nil {
			c.Logger.Err(err).Int32("partition", msg.Partition).Int64("offset", msg.Offset).Msg("Couldn't parse message, skipping.")
			continue
		}

		to := common.HexToAddress(event.Data.To)
		data := common.FromHex(event.Data.Data)

		err = c.Manager.SendTx(session.Context(), &manager.TransactionRequest{ID: event.Data.ID, To: to, Data: data})
		if err != nil {
			c.Logger.Err(err).Msg("Error sending transaction.")
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

	consumer := &Consumer{Logger: logger, EthClient: ethClient, Manager: manager}

	for {
		group.Consume(ctx, []string{topic}, consumer)
		// Context canceled, so quit.
		if ctx.Err() != nil {
			return nil
		}
	}
}
