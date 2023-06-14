package service

import (
	"encoding/json"
	"errors"
	"pickup/datasources/collector"
	"pickup/datasources/kafka"
	"sync"
	"time"

	"git.devucc.name/dependencies/utilities/commons/log"
	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/interfaces"
	"git.devucc.name/dependencies/utilities/models/activity"
	"git.devucc.name/dependencies/utilities/models/engine"
	"git.devucc.name/dependencies/utilities/models/order"
	"git.devucc.name/dependencies/utilities/models/trade"
	"git.devucc.name/dependencies/utilities/types"
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
	nonce              int64
	mutex              *sync.Mutex
}

func NewManagerService(
	k *kafka.Kafka,
	a interfaces.Repository[activity.Activity],
	o interfaces.Repository[order.Order],
	t interfaces.Repository[trade.Trade],
) ManagerService {
	n := int64(0)
	p := []bson.M{{"$sort": bson.M{"createdAt": -1}}, {"$limit": 1}}
	acts := a.Aggregate(p)
	if len(acts) > 0 {
		n = acts[0].Nonce
	}

	return ManagerService{
		kafkaConn:          k,
		activityRepository: a,
		orderRepository:    o,
		tradeRepository:    t,
		nonce:              n,
		mutex:              &sync.Mutex{},
	}
}

func (m *ManagerService) HandlePickup(msg kafkago.Message) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Metrics
	activityId := primitive.NewObjectID()
	go collector.ConsumedMetricCounter(msg.Topic, activityId.Hex())

	// Initialize data
	orders := []*order.Order{}
	trades := []*trade.Trade{}

	nonce := int64(0)

	var data interface{}
	var er *engine.EngineResponse
	var co *order.CancelledOrder

	var err error

	switch msg.Topic {
	case string(types.ENGINE):
		er, nonce, err = m.processEngine(msg.Value)
		orders, trades, err = m.validateOrders(er)
		data = m.activityData(msg.Topic, er)
	case string(types.CANCELLED_ORDER):
		co, nonce, err = m.processCancelledOrders(msg.Value)
		data = m.activityData(msg.Topic, co)
		orders = data.(map[string]interface{})["data"].([]*order.Order)
	}

	if err != nil {
		go m.kafkaConn.Commit(msg)
		go collector.PublishedMetricCounter(msg.Topic, activityId.Hex(), false)
		return nil
	}

	if nonce != m.nonce+1 {
		i := map[string]interface{}{"engine": nonce, "mongo": m.nonce + 1}
		logs.Log.Error().Any("nonce", i).Msg("Invalid nonce!")
		return m.manager(msg.Topic, activityId.Hex())
	}

	if err := m.updateOrders(orders); err != nil {
		return m.manager(msg.Topic, activityId.Hex())
	}

	if err := m.updateTrades(trades); err != nil {
		return m.manager(msg.Topic, activityId.Hex())
	}

	if err := m.kafkaConn.Commit(msg); err != nil {
		return m.manager(msg.Topic, activityId.Hex())
	}

	if err := m.publishSaved(msg); err != nil {
		return m.manager(msg.Topic, activityId.Hex())
	}

	if err := m.insertActivity(activityId, data); err != nil {
		return m.manager(msg.Topic, activityId.Hex())
	}

	go collector.PublishedMetricCounter(msg.Topic, activityId.Hex(), true)

	return nil
}

func (m *ManagerService) processEngine(v []byte) (e *engine.EngineResponse, nonce int64, err error) {
	e = &engine.EngineResponse{}
	if err := json.Unmarshal(v, e); err != nil {
		logs.Log.Error().Err(err).Msg("Failed to parse engine data!")

		if err := m.kafkaConn.Commit(kafkago.Message{Topic: string(types.ENGINE), Value: v}); err != nil {
			return nil, 0, err
		}

		return nil, 0, err
	}

	receivedTime := time.Since(e.CreatedAt).Microseconds()
	logger.Infof("Received from matching engine: %v microseconds", receivedTime)

	if e.Nonce <= 0 {
		logs.Log.Error().Msg("Nonce less than equal zero!")

		if err := m.kafkaConn.Commit(kafkago.Message{Topic: string(types.ENGINE), Value: v}); err != nil {
			return nil, 0, err
		}

		return nil, 0, errors.New("NonceLessThanEqualZero")
	}

	return e, e.Nonce, nil
}

func (m *ManagerService) processCancelledOrders(v []byte) (c *order.CancelledOrder, nonce int64, err error) {
	c = &order.CancelledOrder{}
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

func (m *ManagerService) activityData(t string, r interface{}) (data interface{}) {
	if r == nil {
		return data
	}

	switch t {
	case string(types.ENGINE):
		e := r.(*engine.EngineResponse)
		return e.Matches
	case string(types.CANCELLED_ORDER):
		c := r.(*order.CancelledOrder)
		return map[string]interface{}{
			"query": c.Query,
			"data":  c.Data,
		}
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

func (m *ManagerService) validateOrders(e *engine.EngineResponse) (orders []*order.Order, trades []*trade.Trade, err error) {
	orders = []*order.Order{}
	trades = []*trade.Trade{}

	if e == nil {
		return nil, nil, errors.New("EngineNotFound")
	}

	if e.Status == types.ORDER_REJECTED {
		return nil, nil, errors.New("OrderRejected")
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

	return orders, trades, nil
}

func (m *ManagerService) publishSaved(msg kafkago.Message) error {
	switch msg.Topic {
	case string(types.ENGINE):
		return m.kafkaConn.Publish(kafkago.Message{Topic: string(types.ENGINE_SAVED), Value: msg.Value})
	case string(types.CANCELLED_ORDER):
		return m.kafkaConn.Publish(kafkago.Message{Topic: "CANCELLED_ORDER_SAVED", Value: msg.Value})
	default:
		return errors.New("TopicNotFound")
	}
}

func (m *ManagerService) insertActivity(id primitive.ObjectID, data interface{}) error {
	activity := &activity.Activity{ID: id, Nonce: m.nonce + 1, Data: data, CreatedAt: time.Now()}

	filter := bson.M{"_id": activity.ID}
	update := bson.M{"$set": activity}
	if _, err := m.activityRepository.FindAndModify(filter, update); err != nil {
		return err
	}

	m.nonce += 1

	return nil
}

func (m *ManagerService) manager(topic string, key string) error {
	go collector.PublishedMetricCounter(topic, key, false)

	return nil
}
