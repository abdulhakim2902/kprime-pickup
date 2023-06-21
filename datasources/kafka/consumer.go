package kafka

import (
	"context"

	"git.devucc.name/dependencies/utilities/commons/log"
	"git.devucc.name/dependencies/utilities/types"

	"github.com/segmentio/kafka-go"
)

var logger = log.Logger
var groupID = "gateway-group"

func InitConsumer(url string) *kafka.Reader {
	config := kafka.ReaderConfig{
		Brokers:     []string{url},
		GroupID:     groupID,
		GroupTopics: []string{types.ENGINE.String(), types.CANCELLED_ORDER.String()},
	}

	return kafka.NewReader(config)
}

func (k *Kafka) Subscribe(cb func(kafka.Message) error) {
	go func() {
		for {
			m, e := k.reader.FetchMessage(context.Background())
			if e != nil {
				logger.Errorf("Failed to fetch message!")
				continue
			}

			logger.Infof("Received messages from %v: %v", m.Topic, string(m.Value))

			cb(m)
		}
	}()
}

func (k *Kafka) Commit(msg kafka.Message) error {
	e := k.reader.CommitMessages(context.Background(), msg)
	if e != nil {
		logger.Errorf("Failed to commit message!")
		return e
	}

	return nil
}
