package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

func InitProducer(url string) *kafka.Writer {
	w := kafka.Writer{
		Addr:                   kafka.TCP(url),
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		BatchTimeout:           1000,
	}

	return &w
}

func (k *Kafka) Publish(messages ...kafka.Message) error {
	err := k.writer.WriteMessages(context.Background(), messages...)
	if err != nil {
		logger.Errorf("Failed to write message!")
		return err
	}

	return nil
}
