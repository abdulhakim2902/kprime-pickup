package kafka

import (
	"context"

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
		Brokers:     []string{url},
		GroupID:     groupID,
		GroupTopics: []string{"ENGINE", "CANCELLED_ORDERS"},
	}

	return kafka.NewReader(config)
}

func (k *Kafka) Subscribe(cb func(topic string, data []byte)) {
	go func() {
		for {
			m, e := k.reader.ReadMessage(context.Background())
			if e != nil {
				logger.Errorf("Failed to read message!")
				continue
			}

			logger.Infof("Received messages from %v: %v", m.Topic, string(m.Value))

			go cb(m.Topic, m.Value)
		}
	}()
}
