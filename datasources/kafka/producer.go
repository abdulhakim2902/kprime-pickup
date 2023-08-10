package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"
)

func InitProducer(url string) *kafka.Writer {
	w := kafka.Writer{
		Addr:         kafka.TCP(url),
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 1000,
		Compression:  compress.Lz4,
	}

	return &w
}

func (k *Kafka) Publish(messages ...kafka.Message) error {
	err := k.writer.WriteMessages(context.Background(), messages...)
	if err != nil {
		logger.Errorf("Failed to write message!", err)
		return err
	}

	return nil
}
