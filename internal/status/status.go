package status

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/internal/storage"
	"github.com/DIMO-Network/shared"
	"github.com/Shopify/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
)

type SubmittedMsg struct {
	ID    string         `json:"id"`
	Hash  common.Hash    `json:"hash"`
	Block *storage.Block `json:"block"`
}

type MinedMsg struct {
	ID    string
	Hash  common.Hash
	Block *storage.Block
}

type ConfirmedMsg struct {
	ID         string
	Hash       common.Hash
	Block      *storage.Block
	Logs       []*Log
	Successful bool
}

type Log struct {
	Address common.Address `json:"address"`
	Topics  []common.Hash  `json:"topics"`
	Data    []byte         `json:"data"`
}

type ceLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

type Producer interface {
	Submitted(msg *SubmittedMsg)
	Mined(msg *MinedMsg)
	Confirmed(msg *ConfirmedMsg)
}

type kafkaProducer struct {
	kp     sarama.SyncProducer
	topic  string
	logger *zerolog.Logger
}

type ceTx struct {
	Hash       string  `json:"hash"`
	Successful *bool   `json:"successful,omitempty"`
	Logs       []ceLog `json:"logs,omitempty"`
}

// Just using the same struct for all three event types. Lazy.
type ceData struct {
	RequestID   string `json:"requestId"`
	Type        string `json:"type"`
	Transaction ceTx   `json:"transaction"`
}

func (p *kafkaProducer) Confirmed(msg *ConfirmedMsg) {
	logs := make([]ceLog, len(msg.Logs))
	for i, l := range msg.Logs {
		top := make([]string, len(l.Topics))
		for j, t := range l.Topics {
			top[j] = t.Hex()
		}

		logs[i].Address = hexutil.Encode(l.Address[:])
		logs[i].Topics = top
		logs[i].Data = hexutil.Encode(l.Data)
	}

	event := shared.CloudEvent[ceData]{
		ID:          ksuid.New().String(),
		Source:      "meta-transaction-processor",
		Subject:     msg.ID,
		SpecVersion: "1.0",
		Time:        time.Now(),
		Type:        "zone.dimo.transaction.request.event",
		Data: ceData{
			RequestID: msg.ID,
			Type:      "Confirmed",
			Transaction: ceTx{
				Hash:       msg.Hash.Hex(),
				Successful: &msg.Successful,
				Logs:       logs,
			},
		},
	}

	bs, err := json.Marshal(event)
	if err != nil {
		p.logger.Err(err).Msg("Couldn't marshal confirmed message.")
		return
	}

	p.logger.Info().Interface("event", event).Msg("Emitting event.")

	p.kp.SendMessage(
		&sarama.ProducerMessage{
			Topic: p.topic,
			Value: sarama.ByteEncoder(bs),
		},
	)
}

func (p *kafkaProducer) Submitted(msg *SubmittedMsg) {
	event := shared.CloudEvent[ceData]{
		ID:          ksuid.New().String(),
		Source:      "meta-transaction-processor",
		Subject:     msg.ID,
		SpecVersion: "1.0",
		Time:        time.Now(),
		Type:        "zone.dimo.transaction.request.event",
		Data: ceData{
			RequestID: msg.ID,
			Type:      "Submitted",
			Transaction: ceTx{
				Hash: msg.Hash.Hex(),
			},
		},
	}

	p.logger.Info().Interface("event", event).Msg("Emitting event.")

	bs, err := json.Marshal(event)
	if err != nil {
		p.logger.Err(err).Msg("Couldn't marshal sent message.")
		return
	}

	p.kp.SendMessage(&sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(bs),
	})
}

func (p *kafkaProducer) Mined(msg *MinedMsg) {
	event := shared.CloudEvent[ceData]{
		ID:          ksuid.New().String(),
		Source:      "meta-transaction-processor",
		Subject:     msg.ID,
		SpecVersion: "1.0",
		Time:        time.Now(),
		Type:        "zone.dimo.transaction.request.event",
		Data: ceData{
			RequestID: msg.ID,
			Type:      "Mined",
			Transaction: ceTx{
				Hash: msg.Hash.Hex(),
			},
		},
	}

	p.logger.Info().Interface("event", event).Msg("Emitting event.")

	bs, err := json.Marshal(event)
	if err != nil {
		p.logger.Err(err).Msg("Couldn't marshal mined message.")
		return
	}

	p.kp.SendMessage(
		&sarama.ProducerMessage{
			Topic: p.topic,
			Value: sarama.ByteEncoder(bs),
		},
	)
}

func NewKafka(ctx context.Context, topic string, client sarama.Client, logger *zerolog.Logger) (Producer, error) {
	kp, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		return nil, err
	}

	return &kafkaProducer{kp: kp, topic: topic, logger: logger}, nil
}
