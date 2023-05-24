package service

import (
	"encoding/json"
	"pickup/datasources/kafka"

	"git.devucc.name/dependencies/utilities/interfaces"
	"git.devucc.name/dependencies/utilities/models/activity"
	"git.devucc.name/dependencies/utilities/models/engine"
	"git.devucc.name/dependencies/utilities/models/order"
	"git.devucc.name/dependencies/utilities/models/trade"
	"go.mongodb.org/mongo-driver/bson"

	kafkago "github.com/segmentio/kafka-go"
)

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
	return ManagerService{activityRepository: a, orderRepository: o, tradeRepository: t}
}

func (m *ManagerService) HandlePickup(msg kafkago.Message) {
	// Get latest activity
	activity := &activity.Activity{}
	pipeline := []bson.M{{"$sort": bson.M{"createdAt": -1}}, {"$limit": 1}}
	activities := m.activityRepository.Aggregate(pipeline)

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
		e, err := m.parseEngine(msg.Value)
		if err != nil {
			return
		}

		if e.Nonce <= 0 {
			return
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

		activity.New(e.Matches)

		kNonce = e.Nonce
		mNonce = activity.Nonce
	}

	// Process cancelled orders
	if msg.Topic == "CANCELLED_ORDERS" {
		c, err := m.parseCancelledOrder(msg.Value)
		if err != nil {
			return
		}

		if c.Nonce <= 0 {
			return
		}

		if len(c.Data) > 0 {
			orders = c.Data
		}

		activity.New(c.Data)

		kNonce = c.Nonce
		mNonce = activity.Nonce
	}

	// Compare nonce from mongo with nonce from kafka
	if mNonce != kNonce {
		m.manager()
		return
	}

	for _, o := range orders {
		filter := bson.M{"_id": o.ID}
		update := bson.M{"$set": o}
		_, err := m.orderRepository.FindAndModify(filter, update)
		if err != nil {
			m.manager()
			return
		}
	}

	for _, t := range trades {
		filter := bson.M{"_id": t.ID}
		update := bson.M{"$set": t}
		_, err := m.tradeRepository.FindAndModify(filter, update)
		if err != nil {
			m.manager()
			return
		}
	}

	_, err := m.activityRepository.FindAndModify(bson.M{"_id": activity.ID}, bson.M{"$set": activity})
	if err != nil {
		m.manager()
		return
	}

	m.kafkaConn.Commit(msg)
}

func (m *ManagerService) parseEngine(data []byte) (*engine.EngineResponse, error) {
	e := &engine.EngineResponse{}
	err := json.Unmarshal(data, e)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (m *ManagerService) parseCancelledOrder(data []byte) (*kafka.CancelledOrderData, error) {
	e := &kafka.CancelledOrderData{}
	err := json.Unmarshal(data, e)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (m *ManagerService) manager() {}
