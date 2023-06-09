package service

import (
	"encoding/json"
	"errors"
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
	"go.mongodb.org/mongo-driver/bson/primitive"

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

func (m *ManagerService) HandlePickup(msg kafkago.Message) error {
	// Get latest activity
	activity := &activity.Activity{ID: primitive.NewObjectID(), Nonce: 0, CreatedAt: time.Now()}
	pipeline := []bson.M{{"$sort": bson.M{"createdAt": -1}}, {"$limit": 1}}
	activities := m.activityRepository.Aggregate(pipeline)

	// Reassign nonce from mongodb
	if len(activities) > 0 {
		activity = activities[0]
	}

	// Cancelled detail
	totalCancelled := 0

	// Initialize data
	orders := []*order.Order{}
	trades := []*trade.Trade{}

	nonce := int64(0)
	mongoNonce := activity.Nonce + 1

	var data interface{}
	var er *engine.EngineResponse
	var co *order.CancelledOrder

	switch msg.Topic {
	case "ENGINE":
		er, nonce, _ = m.processEngine(msg.Value)
		orders, trades = m.validateOrders(er)
		data = m.activityData(msg.Topic, er)
	case "CANCELLED_ORDERS":
		co, nonce, _ = m.processCancelledOrders(msg.Value)
		data = m.activityData(msg.Topic, co)
		totalCancelled = co.Total
	default:
		return nil
	}

	if nonce != mongoNonce {
		logs.Log.Error().Msg("Invalid nonce!")
		return m.manager(msg)
	}

	// Update activity detail
	activity.ID = primitive.NewObjectID()
	activity.Nonce = mongoNonce
	activity.Data = data
	activity.CreatedAt = time.Now()

	if err := m.updateCancelledOrders(data, totalCancelled); err != nil {
		return m.manager(msg)
	}

	if err := m.updateOrders(orders); err != nil {
		return m.manager(msg)
	}

	if err := m.updateTrades(trades); err != nil {
		return m.manager(msg)
	}

	if err := m.kafkaConn.Commit(msg); err != nil {
		return m.manager(msg)
	}

	if err := m.kafkaConn.Publish(kafkago.Message{Topic: "ENGINE_SAVED", Value: msg.Value}); err != nil && msg.Topic == "ENGINE" {
		return m.manager(msg)
	}

	if _, err := m.activityRepository.FindAndModify(bson.M{"_id": activity.ID}, bson.M{"$set": activity}); err != nil {
		return m.manager(msg)
	}

	return nil
}

func (m *ManagerService) processEngine(v []byte) (*engine.EngineResponse, int64, error) {
	e := &engine.EngineResponse{}
	if err := json.Unmarshal(v, e); err != nil {
		logs.Log.Error().Err(err).Msg("Failed to parse engine data!")

		if err := m.kafkaConn.Commit(kafkago.Message{Topic: "ENGINE", Value: v}); err != nil {
			return nil, 0, err
		}

		return nil, 0, err
	}

	receivedTime := time.Since(e.CreatedAt).Microseconds()
	logger.Infof("Received from matching engine: %v microseconds", receivedTime)

	if e.Nonce <= 0 {
		logs.Log.Error().Msg("Nonce less than equal zero!")

		if err := m.kafkaConn.Commit(kafkago.Message{Topic: "ENGINE", Value: v}); err != nil {
			return nil, 0, err
		}

		return nil, 0, errors.New("NonceLessThanEqualZero")
	}

	return e, e.Nonce, nil
}

func (m *ManagerService) processCancelledOrders(v []byte) (*order.CancelledOrder, int64, error) {
	c := &order.CancelledOrder{}
	if err := json.Unmarshal(v, c); err != nil {
		logs.Log.Error().Err(err).Msg("Failed to parse cancelled orders data!")
		if err := m.kafkaConn.Commit(kafkago.Message{Topic: "CANCELLED_ORDERS", Value: v}); err != nil {
			return nil, 0, err
		}

		return nil, 0, err
	}

	if c.Nonce <= 0 {
		logs.Log.Error().Msg("Nonce less than equal zero!")

		if err := m.kafkaConn.Commit(kafkago.Message{Topic: "CANCELLED_ORDERS", Value: v}); err != nil {
			return nil, 0, err
		}

		return nil, 0, errors.New("NonceLessThanEqualZero")
	}

	return c, c.Nonce, nil
}

func (m *ManagerService) updateCancelledOrders(f interface{}, n int) error {
	if n <= 0 {
		return nil
	}

	filter := map[string]interface{}{}

	raw := f.(map[string]interface{})
	for k, v := range raw {
		if k == "clOrdId" {
			continue
		}

		filter[k] = v
	}

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
	return nil
}

func (m *ManagerService) activityData(t string, r interface{}) (data interface{}) {
	if r == nil {
		return data
	}

	switch t {
	case "ENGINE":
		e := r.(*engine.EngineResponse)
		return e.Matches
	case "CANCELLED_ORDERS":
		c := r.(*order.CancelledOrder)
		return c.Data
	default:
		return r
	}
}

func (m *ManagerService) updateOrders(o []*order.Order) error {
	for _, order := range o {
		filter := bson.M{"_id": order.ID}
		update := bson.M{"$set": order}

		if _, err := m.orderRepository.FindAndModify(filter, update); err != nil {
			return err
		}
	}

	return nil
}

func (m *ManagerService) updateTrades(t []*trade.Trade) error {
	for _, trade := range t {
		filter := bson.M{"_id": trade.ID}
		update := bson.M{"$set": trade}

		if _, err := m.tradeRepository.FindAndModify(filter, update); err != nil {
			return err
		}
	}

	return nil
}

func (m *ManagerService) validateOrders(e *engine.EngineResponse) (orders []*order.Order, trades []*trade.Trade) {
	if e == nil {
		return orders, trades
	}

	if e.Status == types.ORDER_REJECTED {
		return orders, trades
	}

	if len(e.Matches.MakerOrders) > 0 {
		orders = e.Matches.MakerOrders
	}

	if len(e.Matches.Trades) > 0 {
		trades = e.Matches.Trades
	}

	if e.Matches.TakerOrder != nil {
		orders = append(orders, e.Matches.TakerOrder)
	}

	return orders, trades
}

func (m *ManagerService) manager(msg kafkago.Message) error {
	if err := m.kafkaConn.Commit(msg); err != nil {
		return err
	}

	if err := m.kafkaConn.Publish(kafkago.Message{Topic: msg.Topic, Value: msg.Value}); err != nil {
		logs.Log.Err(err).Msg("Failed to publish")
	}

	return nil
}
