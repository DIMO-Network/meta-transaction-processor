package status

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/storage"
	"github.com/DIMO-Network/shared"
	"github.com/Shopify/sarama"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/segmentio/ksuid"
)

type CreatedMsg struct {
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
	Created(msg *CreatedMsg)
	Mined(msg *MinedMsg)
	Confirmed(msg *ConfirmedMsg)
}

type kafkaProducer struct {
	kp    sarama.SyncProducer
	topic string
}

type ceData struct {
	ID         string   `json:"id"`
	Hash       string   `json:"hash"`
	Successful *bool    `json:"successful,omitempty"`
	Logs       []*ceLog `json:"logs,omitempty"`
}

func (p *kafkaProducer) Confirmed(msg *ConfirmedMsg) {
	var logs []*ceLog
	if msg.Successful {
		logs = make([]*ceLog, 0)
		for _, l := range msg.Logs {
			top := make([]string, 0)
			for _, s := range l.Topics {
				top = append(top, s.Hex())
			}
			cel := &ceLog{
				Address: hexutil.Encode(l.Address[:]),
				Topics:  top,
				Data:    hexutil.Encode(l.Data),
			}
			logs = append(logs, cel)
		}
	}
	event := shared.CloudEvent[ceData]{
		ID:          ksuid.New().String(),
		Source:      "meta-transaction-processor",
		Subject:     msg.ID,
		SpecVersion: "1.0",
		Time:        time.Now(),
		Data: ceData{
			ID:         msg.ID,
			Hash:       msg.Hash.Hex(),
			Successful: &msg.Successful,
			Logs:       logs,
		},
	}

	bs, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}

	p.kp.SendMessage(&sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(bs),
	})
}

func (p *kafkaProducer) Created(msg *CreatedMsg) {
	event := shared.CloudEvent[ceData]{
		ID:          ksuid.New().String(),
		Source:      "meta-transaction-processor",
		Subject:     msg.ID,
		SpecVersion: "1.0",
		Time:        time.Now(),
		Data: ceData{
			ID:   msg.ID,
			Hash: msg.Hash.Hex(),
		},
	}

	bs, err := json.Marshal(event)
	if err != nil {
		panic(err)
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
		Data: ceData{
			ID:   msg.ID,
			Hash: msg.Hash.Hex(),
		},
	}

	bs, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}

	p.kp.SendMessage(&sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.ByteEncoder(bs),
	})
}

func NewKafka(ctx context.Context, topic string, client sarama.Client) (Producer, error) {
	kp, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		return nil, err
	}

	return &kafkaProducer{kp: kp, topic: topic}, nil
}
