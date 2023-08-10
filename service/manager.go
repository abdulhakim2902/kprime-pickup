package service

import (
	"encoding/json"
	"errors"
	"pickup/datasources/collector"
	"pickup/datasources/kafka"
	"sync"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/log"
	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/activity"
	model "github.com/Undercurrent-Technologies/kprime-utilities/models/kafka"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/order"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/trade"
	"github.com/Undercurrent-Technologies/kprime-utilities/models/user"
	"github.com/Undercurrent-Technologies/kprime-utilities/repository/mongodb"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	kafkago "github.com/segmentio/kafka-go"
)

var logger = log.Logger

type PickupResult struct {
	orders      []*order.Order
	trades      []*trade.Trade
	data        interface{}
	nonce       int64
	kafkaOffset int64
}

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
	acts, _ := r.Activity.Aggregate(p)
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

func (m *ManagerService) HandlePickup(msg kafkago.Message) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Metrics
	activityId := primitive.NewObjectID()
	go m.requestDurations.StartRequestDuration(msg.Topic, activityId.Hex())

	// Initialize data
	var res *PickupResult
	var err error

	switch msg.Topic {
	case types.ENGINE.String():
		res, err = m.processEngine(msg)
	case types.CANCELLED_ORDER.String():
		res, err = m.processCancelledOrders(msg)
	default:
		return
	}

	if err != nil {
		logs.Log.Error().Err(err).Msg("Failed processing order")
		go m.kafkaConn.Commit(msg)
		go m.requestDurations.EndRequestDuration(msg.Topic, activityId.Hex(), false)
		return
	}

	m.updateOrders(res.orders)
	m.updateTrades(res.trades)
	m.kafkaConn.Commit(msg)
	m.insertActivity(activityId, res)
	m.publishSaved(msg)

	go m.requestDurations.EndRequestDuration(msg.Topic, activityId.Hex(), true)

	return
}

func (m *ManagerService) processEngine(msg kafkago.Message) (res *PickupResult, err error) {
	v := msg.Value
	e := &model.EngineResponse{}
	if err := json.Unmarshal(v, e); err != nil {
		return nil, err
	}

	if e.Nonce <= 0 {
		return nil, errors.New("NonceLessThanEqualZero")
	}

	if e.Status == types.ORDER_REJECTED {
		return nil, errors.New("OrderRejected")
	}

	res = &PickupResult{
		orders:      []*order.Order{},
		trades:      []*trade.Trade{},
		data:        e.Matches,
		nonce:       e.Nonce,
		kafkaOffset: msg.Offset,
	}

	if len(e.Matches.MakerOrders) > 0 {
		res.orders = e.Matches.MakerOrders
	}

	if len(e.Matches.Trades) > 0 {
		res.trades = e.Matches.Trades
	}

	if e.Matches.TakerOrder != nil {
		res.orders = append(res.orders, e.Matches.TakerOrder)
	}

	return res, nil
}

func (m *ManagerService) processCancelledOrders(msg kafkago.Message) (res *PickupResult, err error) {
	v := msg.Value
	c := &model.CancelledOrder{}
	if err := json.Unmarshal(v, c); err != nil {
		return nil, err
	}

	if c.Nonce <= 0 {
		return nil, errors.New("NonceLessThanEqualZero")
	}

	res = &PickupResult{
		orders:      c.Data,
		trades:      []*trade.Trade{},
		nonce:       c.Nonce,
		kafkaOffset: msg.Offset,
		data: map[string]interface{}{
			"query": c.Query,
		},
	}

	return res, nil
}

func (m *ManagerService) updateOrders(o []*order.Order) {
	for _, order := range o {
		filter := bson.M{"_id": order.ID}
		update := bson.M{"$set": order}

		m.repositories.Order.FindAndModify(filter, update)
	}
}

func (m *ManagerService) updateTrades(t []*trade.Trade) error {
	for _, trade := range t {
		filter := bson.M{"_id": trade.ID}
		update := bson.M{"$set": trade}

		if _, err := m.repositories.Trade.FindAndModify(filter, update); err != nil {
			continue
		}

		if trade.Status == types.FAILED {
			continue
		}

		m.updateUserCollateral(trade, trade.Taker)
		m.updateUserCollateral(trade, trade.Maker)
	}

	return nil
}

func (m *ManagerService) updateUserCollateral(t *trade.Trade, us *trade.User) {
	s := us.Side

	i := t.OrderCode()
	f := bson.M{"_id": us.UserID}
	u := m.repositories.User.FindOne(f)
	if u == nil {
		logs.Log.Error().Any("userId", us.UserID).Msg("User not found")
		return
	}

	p := t.GetAmount().Mul(t.GetPrice())

	for _, bal := range u.Collaterals.Balances {
		if bal.Currency != "USD" {
			continue
		}

		balance := bal.GetAmount()
		if s == types.BUY {
			bal.Amount = balance.Sub(p.Add(us.Fee.GetAmount())).String()
		} else {
			bal.Amount = balance.Add(p.Sub(us.Fee.GetAmount())).String()
		}

		break
	}

	exist := false
	for _, con := range u.Collaterals.Contracts {
		if con.InstrumentName == i {
			exist = true

			if s == types.BUY {
				con.Amount = con.GetAmount().Add(t.GetAmount()).String()
			} else {
				con.Amount = con.GetAmount().Sub(t.GetAmount()).String()
			}

			break
		}
	}

	if !exist {
		newContract := &user.Contract{InstrumentName: i}
		if s == types.BUY {
			newContract.Amount = t.Amount
		} else {
			newContract.Amount = "-" + t.Amount
		}

		u.Collaterals.Contracts = append(u.Collaterals.Contracts, newContract)
	}

	m.repositories.User.FindAndModify(f, bson.M{"$set": u})
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

func (m *ManagerService) insertActivity(id primitive.ObjectID, res *PickupResult) error {
	activity := &activity.Activity{ID: id, Nonce: res.nonce, KafkaOffset: res.kafkaOffset, Data: res.data, CreatedAt: time.Now()}

	filter := bson.M{"_id": activity.ID}
	update := bson.M{"$set": activity}
	if _, err := m.repositories.Activity.FindAndModify(filter, update); err != nil {
		return err
	}

	m.nonce += 1

	return nil
}
