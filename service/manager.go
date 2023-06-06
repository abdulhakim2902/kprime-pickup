package service

import (
	"encoding/json"
	"pickup/datasources/kafka"
	"time"

	"git.devucc.name/dependencies/utilities/commons/log"
	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/interfaces"
	"git.devucc.name/dependencies/utilities/models/activity"
	"git.devucc.name/dependencies/utilities/models/engine"
	"git.devucc.name/dependencies/utilities/models/order"
	"git.devucc.name/dependencies/utilities/models/trade"
	"git.devucc.name/dependencies/utilities/types"
	"git.devucc.name/dependencies/utilities/types/cancelled_reason"
	"go.mongodb.org/mongo-driver/bson"

	kafkago "github.com/segmentio/kafka-go"
)

var logger = log.Logger

type ManagerService struct {
	kafkaConn          *kafka.Kafka
	activityRepository interfaces.Repository[activity.Activity]
	orderRepository    interfaces.Repository[order.Order]
	tradeRepository    interfaces.Repository[trade.Trade]
}

func NewManagerService(
	k *kafka.Kafka,
	a interfaces.Repository[activity.Activity],
	o interfaces.Repository[order.Order],
	t interfaces.Repository[trade.Trade],
) ManagerService {
	return ManagerService{kafkaConn: k, activityRepository: a, orderRepository: o, tradeRepository: t}
}

func (m *ManagerService) HandlePickup(msg kafkago.Message) {
	// Get latest activity
	activity := &activity.Activity{}
	pipeline := []bson.M{{"$sort": bson.M{"createdAt": -1}}, {"$limit": 1}}
	activities := m.activityRepository.Aggregate(pipeline)

	// Cancelled detail
	totalCancelled := 0
	filter := bson.M{}

	// Initialize data
	orders := []*order.Order{}
	trades := []*trade.Trade{}
	kNonce := int64(0)
	mNonce := int64(0)

	// Reassign nonce from mongodb
	if len(activities) > 0 {
		activity = activities[0]
		mNonce = activity.Nonce
	}

	// Proses Engine response
	if msg.Topic == "ENGINE" {
		e := &engine.EngineResponse{}
		if err := json.Unmarshal(msg.Value, e); err != nil {
			logs.Log.Error().Err(err).Msg("Failed to parse engine data!")
			return
		}

		receivedTime := time.Since(e.CreatedAt).Microseconds()
		logger.Infof("Received from matching engine: %v microseconds", receivedTime)

		if e.Nonce <= 0 {
			logs.Log.Error().Msg("Nonce less than equal zero!")
			return
		}

		if e.Status != types.ORDER_REJECTED {
			if len(e.Matches.MakerOrders) > 0 {
				orders = e.Matches.MakerOrders
			}

			if len(e.Matches.Trades) > 0 {
				trades = e.Matches.Trades
			}

			if e.Matches.TakerOrder != nil {
				orders = append(orders, e.Matches.TakerOrder)
			}
		}

		activity.New(e.Matches)

		kNonce = e.Nonce
		mNonce = activity.Nonce
	}

	// Process cancelled orders
	if msg.Topic == "CANCELLED_ORDERS" {
		c := &order.CancelledOrder{}
		if err := json.Unmarshal(msg.Value, c); err != nil {
			logs.Log.Error().Err(err).Msg("Failed to parse cancelled orders data!")
			return
		}

		if c.Nonce <= 0 {
			logs.Log.Error().Msg("Nonce less than equal zero!")
			return
		}

		activity.New(c.Data)

		kNonce = c.Nonce
		mNonce = activity.Nonce

		totalCancelled = c.Total
		filter = c.Data.(bson.M)
	}

	// Compare nonce from mongo with nonce from kafka
	if mNonce != kNonce {
		m.manager()
		logs.Log.Error().Msg("Invalid nonce!")
		return
	}

	if totalCancelled > 0 {
		now := time.Now()
		update := bson.M{
			"$set": bson.M{
				"status":          types.CANCELLED,
				"cancelledReason": cancelled_reason.USER_REQUEST,
				"updatedAt":       now,
				"cancelledAt":     now,
			},
		}
		m.orderRepository.UpdateAll(filter, update)
	}

	for _, o := range orders {
		filter := bson.M{"_id": o.ID}
		update := bson.M{"$set": o}
		_, err := m.orderRepository.FindAndModify(filter, update)
		if err != nil {
			m.manager()
			logs.Log.Error().Err(err).Msg("Failed to create order!")
			return
		}
	}

	for _, t := range trades {
		filter := bson.M{"_id": t.ID}
		update := bson.M{"$set": t}
		_, err := m.tradeRepository.FindAndModify(filter, update)
		if err != nil {
			m.manager()
			logs.Log.Error().Err(err).Msg("Failed to create trade!")
			return
		}
	}

	_, err := m.activityRepository.FindAndModify(bson.M{"_id": activity.ID}, bson.M{"$set": activity})
	if err != nil {
		m.manager()
		logs.Log.Error().Err(err).Msg("Failed to create activity!")
		return
	}

	if err := m.kafkaConn.Commit(msg); err != nil {
		m.manager()
		logs.Log.Error().Err(err).Msg("Failed to commit message!")
		return
	}

	if err := m.kafkaConn.Publish(kafkago.Message{
		Topic: "ENGINE_SAVED",
		Value: msg.Value,
	}); err != nil {
		logs.Log.Error().Err(err).Msg("Failed to publish message!")
	}
}

func (m *ManagerService) parseEngine(data []byte) (*engine.EngineResponse, error) {
	e := &engine.EngineResponse{}
	err := json.Unmarshal(data, e)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (m *ManagerService) parseCancelledOrder(data []byte) (*order.CancelledOrder, error) {
	e := &order.CancelledOrder{}
	err := json.Unmarshal(data, e)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (m *ManagerService) manager() {}
