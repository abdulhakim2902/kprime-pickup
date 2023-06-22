package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"pickup/datasources/collector"
	"pickup/datasources/kafka"
	"sync"
	"time"

	"git.devucc.name/dependencies/utilities/commons/log"
	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/models/activity"
	"git.devucc.name/dependencies/utilities/models/engine"
	"git.devucc.name/dependencies/utilities/models/order"
	"git.devucc.name/dependencies/utilities/models/trade"
	"git.devucc.name/dependencies/utilities/models/user"
	"git.devucc.name/dependencies/utilities/repository/mongodb"
	"git.devucc.name/dependencies/utilities/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/shopspring/decimal"
)

var logger = log.Logger

type ManagerService struct {
	kafkaConn        *kafka.Kafka
	repositories     *mongodb.Repositories
	nonce            int64
	requestDurations collector.RequestDurations
	mutex            *sync.Mutex
}

func NewManagerService(k *kafka.Kafka, r *mongodb.Repositories) ManagerService {
	n := int64(0)
	p := []bson.M{{"$sort": bson.M{"createdAt": -1}}, {"$limit": 1}}
	acts := r.Activity.Aggregate(p)
	if len(acts) > 0 {
		n = acts[0].Nonce
	}

	rd := collector.RequestDurations{
		RequestDurations: map[string]collector.RequestDuration{},
		Mutex:            &sync.Mutex{},
	}

	return ManagerService{
		kafkaConn:        k,
		repositories:     r,
		nonce:            n,
		requestDurations: rd,
		mutex:            &sync.Mutex{},
	}
}

func (m *ManagerService) HandlePickup(msg kafkago.Message) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Metrics
	activityId := primitive.NewObjectID()
	go m.requestDurations.StartRequestDuration(msg.Topic, activityId.Hex())

	// Initialize data
	orders := []*order.Order{}
	trades := []*trade.Trade{}

	nonce := int64(0)

	var data interface{}
	var er *engine.EngineResponse
	var co *order.CancelledOrder

	var err error

	switch msg.Topic {
	case types.ENGINE.String():
		er, nonce, err = m.processEngine(msg)
		orders, trades, err = m.validateOrders(er)
		data = m.activityData(msg.Topic, er)
	case types.CANCELLED_ORDER.String():
		co, nonce, err = m.processCancelledOrders(msg)
		data = m.activityData(msg.Topic, co)
		orders = data.(map[string]interface{})["data"].([]*order.Order)
	}

	if err != nil {
		go m.kafkaConn.Commit(msg)
		go m.requestDurations.EndRequestDuration(msg.Topic, activityId.Hex(), false)
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

	go m.requestDurations.EndRequestDuration(msg.Topic, activityId.Hex(), true)

	return nil
}

func (m *ManagerService) processEngine(msg kafkago.Message) (e *engine.EngineResponse, nonce int64, err error) {
	v := msg.Value
	e = &engine.EngineResponse{}
	if err := json.Unmarshal(v, e); err != nil {
		logs.Log.Error().Err(err).Msg("Failed to parse engine data!")

		if err := m.kafkaConn.Commit(msg); err != nil {
			return nil, 0, err
		}

		return nil, 0, err
	}

	receivedTime := time.Since(e.CreatedAt).Microseconds()
	logger.Infof("Received from matching engine: %v microseconds", receivedTime)

	if e.Nonce <= 0 {
		logs.Log.Error().Msg("Nonce less than equal zero!")

		if err := m.kafkaConn.Commit(msg); err != nil {
			return nil, 0, err
		}

		return nil, 0, errors.New("NonceLessThanEqualZero")
	}

	return e, e.Nonce, nil
}

func (m *ManagerService) processCancelledOrders(msg kafkago.Message) (c *order.CancelledOrder, nonce int64, err error) {
	v := msg.Value
	c = &order.CancelledOrder{}
	if err := json.Unmarshal(v, c); err != nil {
		logs.Log.Error().Err(err).Msg("Failed to parse cancelled orders data!")
		if err := m.kafkaConn.Commit(msg); err != nil {
			return nil, 0, err
		}

		return nil, 0, err
	}

	if c.Nonce <= 0 {
		logs.Log.Error().Msg("Nonce less than equal zero!")

		if err := m.kafkaConn.Commit(msg); err != nil {
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
	case types.ENGINE.String():
		e := r.(*engine.EngineResponse)
		return e.Matches
	case types.CANCELLED_ORDER.String():
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

		if _, err := m.repositories.Order.FindAndModify(filter, update); err != nil {
			return err
		}
	}

	return nil
}

func (m *ManagerService) updateTrades(t []*trade.Trade) error {
	for _, trade := range t {
		filter := bson.M{"_id": trade.ID}
		update := bson.M{"$set": trade}

		if _, err := m.repositories.Trade.FindAndModify(filter, update); err != nil {
			return err
		}

		if trade.Status == types.FAILED {
			continue
		}

		m.updateUserCollateral(trade, trade.Taker)
		m.updateUserCollateral(trade, trade.Maker)
	}

	return nil
}

func (m *ManagerService) updateUserCollateral(t *trade.Trade, us trade.User) {
	s := us.Side

	o, _ := primitive.ObjectIDFromHex(us.UserID)

	i := t.OrderCode()
	f := bson.M{"_id": o}
	u := m.repositories.User.FindOne(f)
	a := decimal.NewFromFloat(t.Amount)
	p := a.Mul(decimal.NewFromFloat(t.Price))

	for _, bal := range u.Collaterals.Balances {
		if bal.Currency == "USD" {
			balance := bal.GetAmount()
			if s == types.BUY {
				bal.Amount = balance.Add(p).String()
			} else {
				bal.Amount = balance.Sub(p).String()
			}

			break
		}
	}

	exist := false
	for _, con := range u.Collaterals.Contracts {
		if con.InstrumentName == i {
			exist = true

			if s == types.BUY {
				con.Amount = con.GetAmount().Sub(a).String()
			} else {
				con.Amount = con.GetAmount().Add(a).String()
			}

			break
		}
	}

	if !exist {
		newContract := &user.Contract{InstrumentName: i}
		if s == types.BUY {
			newContract.Amount = fmt.Sprintf("-%f", t.Amount)
		} else {
			newContract.Amount = fmt.Sprintf("%f", t.Amount)
		}

		u.Collaterals.Contracts = append(u.Collaterals.Contracts, newContract)
	}

	m.repositories.User.FindAndModify(f, bson.M{"$set": u})
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
	case types.ENGINE.String():
		return m.kafkaConn.Publish(kafkago.Message{Topic: types.ENGINE_SAVED.String(), Value: msg.Value})
	case types.CANCELLED_ORDER.String():
		return m.kafkaConn.Publish(kafkago.Message{Topic: types.CANCELLED_ORDER_SAVED.String(), Value: msg.Value})
	default:
		return errors.New("TopicNotFound")
	}
}

func (m *ManagerService) insertActivity(id primitive.ObjectID, data interface{}) error {
	activity := &activity.Activity{ID: id, Nonce: m.nonce + 1, Data: data, CreatedAt: time.Now()}

	filter := bson.M{"_id": activity.ID}
	update := bson.M{"$set": activity}
	if _, err := m.repositories.Activity.FindAndModify(filter, update); err != nil {
		return err
	}

	m.nonce += 1

	return nil
}

func (m *ManagerService) manager(topic string, key string) error {
	go m.requestDurations.EndRequestDuration(topic, key, false)

	return nil
}
