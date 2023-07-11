package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/log"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"

	"github.com/segmentio/kafka-go"
)

var logger = log.Logger
var groupID = "gateway-group"

func InitConsumer(url string) *kafka.Reader {
	config := kafka.ReaderConfig{
		Brokers:        []string{url},
		GroupID:        groupID,
		GroupTopics:    []string{types.ENGINE.String(), types.CANCELLED_ORDER.String()},
		CommitInterval: 10 * time.Millisecond,
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

			go cb(m)
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

var status = false
var startOffset int64
var startTime time.Time

func (k *Kafka) start(msg kafka.Message) {
	if string(msg.Key) != "start" {
		return
	}

	startTime = time.Now()
	status = true
	startOffset = msg.Offset
}

func (k *Kafka) end(msg kafka.Message) {
	total := msg.Offset - startOffset + 1
	if total%1000 != 0 {
		return
	}

	if status {
		fmt.Printf("consumer consumed %d messages at speed %.2f/s\n", total, float64(total)/time.Since(startTime).Seconds())
	}

	if string(msg.Key) == "end" {
		status = false
	}
}
