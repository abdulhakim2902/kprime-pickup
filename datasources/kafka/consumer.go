package kafka

import (
	"context"
	"encoding/json"

	"git.devucc.name/dependencies/utilities/commons/log"
	"git.devucc.name/dependencies/utilities/models/engine"
	"github.com/segmentio/kafka-go"
)

type CancelledOrderData = map[string]interface{}

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

func (k *Kafka) Subscribe(cb func(e *engine.EngineResponse, c *CancelledOrderData) int64) {
	go func() {
		for {
			m, e := k.reader.ReadMessage(context.Background())
			if e != nil {
				logger.Errorf("Failed to read message!")
				continue
			}

			logger.Infof("Received messages from %v: %v", m.Topic, string(m.Value))

			var engine *engine.EngineResponse
			var cancelledOrderData *CancelledOrderData

			if m.Topic == "ENGINE" {
				err := json.Unmarshal(m.Value, engine)
				if err != nil {
					logger.Errorf("Failed to parse message in", m.Topic)
					engine = nil
				}
			}

			if m.Topic == "CANCELLED_ORDERS" {
				err := json.Unmarshal(m.Value, cancelledOrderData)
				if err != nil {
					logger.Errorf("Failed to parse message in", m.Topic)
					cancelledOrderData = nil
				}
			}

			go cb(engine, cancelledOrderData)
		}
	}()
}
