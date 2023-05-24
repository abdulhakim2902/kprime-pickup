package kafka

import (
	"context"
	"time"

	"git.devucc.name/dependencies/utilities/commons/log"
	"git.devucc.name/dependencies/utilities/models/order"

	"github.com/segmentio/kafka-go"
)

type CancelledOrderData struct {
	Query []*order.Order `json:"query"`
	Data  []*order.Order `json:"data"`
	Nonce int64          `json:"nonce"`
}

var logger = log.Logger
var groupID = "gateway-group"

func InitConsumer(url string) *kafka.Reader {
	config := kafka.ReaderConfig{
		Brokers:        []string{url},
		GroupID:        groupID,
		GroupTopics:    []string{"ENGINE", "CANCELLED_ORDERS"},
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
	}

	return kafka.NewReader(config)
}

func (k *Kafka) Subscribe(cb func(kafka.Message)) {
	go func() {
		for {
			m, e := k.reader.FetchMessage(context.Background())
			if e != nil {
				logger.Errorf("Failed to fetch message!")
				continue
			}

			logger.Infof("Received messages from %v: %v", m.Topic, string(m.Value))

			go cb(m)
		}
	}()
}

func (k *Kafka) Commit(msg kafka.Message) {
	e := k.reader.CommitMessages(context.Background(), msg)
	if e != nil {
		logger.Errorf("Failed to commit message!")
	}
}
