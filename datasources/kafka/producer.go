package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"
)

func InitProducer(url string) *kafka.Writer {
	w := kafka.Writer{
		Addr:         kafka.TCP(url),
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		ReadTimeout:  20 * time.Millisecond,
		WriteTimeout: 20 * time.Millisecond,
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
