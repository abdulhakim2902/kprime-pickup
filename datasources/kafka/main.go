package kafka

import (
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"github.com/segmentio/kafka-go"
)

type Kafka struct {
	reader *kafka.Reader
	writer *kafka.Writer
}

func InitConnection(url string, topics ...types.Topic) (*Kafka, error) {
	logger.Infof("Kafka connecting...")
	conn, err := kafka.Dial("tcp", url)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// List topics
	partitions, err := conn.ReadPartitions()
	if err != nil {
		return nil, err
	}

	controller, err := conn.Controller()
	if err != nil {
		return nil, err
	}

	controllerConn, err := kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return nil, err
	}
	defer controllerConn.Close()

	topicConfig := make([]kafka.TopicConfig, len(topics))

	for _, t := range topics {
		exist := false
		topic := t.String()
		for _, p := range partitions {
			if p.Topic == topic {
				exist = true
				break
			}
		}

		if !exist {
			topicConfig = append(topicConfig, kafka.TopicConfig{
				Topic:             topic,
				NumPartitions:     1,
				ReplicationFactor: 1,
			})
		}
	}

	// Create non existing topics
	_ = controllerConn.CreateTopics(topicConfig...)

	k := &Kafka{}
	k.reader = InitConsumer(url)
	k.writer = InitProducer(url)

	logger.Infof("Kafka connected!")

	return k, nil
}

func (k *Kafka) CloseConnection() {
	sigchnl := make(chan os.Signal, 1)
	signal.Notify(sigchnl, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigchnl
		if sig == syscall.SIGTERM || sig == syscall.SIGINT {
			logger.Infof("Close kafka connection...")
			err := k.reader.Close()
			if err != nil {
				logger.Errorf(err.Error())
			}

			err = k.writer.Close()
			if err != nil {
				logger.Errorf(err.Error())
			}

			logger.Infof("Kafka connection closed!")
			close(sigchnl)
			os.Exit(0)
		}
	}()
}
