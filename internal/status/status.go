package status

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DIMO-Network/shared"
	"github.com/IBM/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/rs/zerolog"
	"github.com/segmentio/ksuid"
)

type SubmittedMsg struct {
	ID   string      `json:"id"`
	Hash common.Hash `json:"hash"`
}

type MinedMsg struct {
	ID   string
	Hash common.Hash
}

type ConfirmedMsg struct {
	ID         string
	Hash       common.Hash
	Logs       []*Log
	Successful bool
}

type Log struct {
	Address common.Address `json:"address"`
	Topics  []common.Hash  `json:"topics"`
	Data    hexutil.Bytes  `json:"data"`
}

//go:generate mockgen -source status.go -destination ../mocks/status_client.go -package mocks
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

type tx struct {
	Hash       common.Hash `json:"hash"`
	Successful *bool       `json:"successful,omitempty"`
	Logs       []*Log      `json:"logs,omitempty"`
}

// Just using the same struct for all three event types. Lazy.
type ceData struct {
	RequestID   string `json:"requestId"`
	Type        string `json:"type"`
	Transaction tx     `json:"transaction"`
}

func (p *kafkaProducer) Confirmed(msg *ConfirmedMsg) {
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
			Transaction: tx{
				Hash:       msg.Hash,
				Successful: &msg.Successful,
				Logs:       msg.Logs,
			},
		},
	}

	bs, err := json.Marshal(event)
	if err != nil {
		p.logger.Err(err).Msg("Couldn't marshal confirmed message.")
		return
	}

	_, _, err = p.kp.SendMessage(
		&sarama.ProducerMessage{
			Topic: p.topic,
			Value: sarama.ByteEncoder(bs),
		},
	)

	if err != nil {
		p.logger.Err(err).Str("requestId", msg.ID).Str("type", "Confirmed").Msg("Failed sending status update.")
	}
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
			Transaction: tx{
				Hash: msg.Hash,
			},
		},
	}

	bs, err := json.Marshal(event)
	if err != nil {
		p.logger.Err(err).Msg("Couldn't marshal sent message.")
		return
	}

	_, _, err = p.kp.SendMessage(
		&sarama.ProducerMessage{
			Topic: p.topic,
			Value: sarama.ByteEncoder(bs),
		},
	)

	if err != nil {
		p.logger.Err(err).Str("requestId", msg.ID).Str("type", "Submitted").Msg("Failed sending status update.")
	}
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
			Transaction: tx{
				Hash: msg.Hash,
			},
		},
	}

	bs, err := json.Marshal(event)
	if err != nil {
		p.logger.Err(err).Msg("Couldn't marshal mined message.")
		return
	}

	_, _, err = p.kp.SendMessage(
		&sarama.ProducerMessage{
			Topic: p.topic,
			Value: sarama.ByteEncoder(bs),
		},
	)

	if err != nil {
		p.logger.Err(err).Str("requestId", msg.ID).Str("type", "Mined").Msg("Failed sending status update.")
	}
}

func NewKafka(ctx context.Context, topic string, client sarama.Client, logger *zerolog.Logger) (Producer, error) {
	kp, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		return nil, err
	}

	return &kafkaProducer{kp: kp, topic: topic, logger: logger}, nil
}
