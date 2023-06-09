package collector

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	labels = []string{"topic"}

	IncomingCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "incoming_counter",
		Help: "The total number of incoming request",
	}, labels)

	SuccessCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "success_counter",
		Help: "The total number of success response",
	}, labels)

	FailureCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "failure_counter",
		Help: "The total number of error response",
	}, labels)

	RequestDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_duration",
		Help: "The total number of request duration",
	}, labels)
)

type RequestDuration struct {
	Topic         string
	StartDuration uint64
	EndDuration   uint64
}

var (
	RequestDurations      map[string]RequestDuration
	RequestDurationsMutex sync.RWMutex
)

func genRequestDurationKey(userId, clOrdID string) string {
	return fmt.Sprintf("%s-%s", clOrdID, userId)
}

func cleanUpDuration(key string) {
	RequestDurationsMutex.RLock()
	defer RequestDurationsMutex.RUnlock()

	delete(RequestDurations, key)
}

func StartRequestDuration(userId, clOrdID string, request RequestDuration) {
	if RequestDurations == nil {
		RequestDurations = make(map[string]RequestDuration)
	}

	key := genRequestDurationKey(userId, clOrdID)
	start := uint64(time.Now().UnixMicro())
	request.StartDuration = start

	// Add duration
	RequestDurationsMutex.RLock()
	defer RequestDurationsMutex.RUnlock()

	RequestDurations[key] = request
}

func EndRequestDuration(userId, clOrdID, topic string) {
	key := genRequestDurationKey(userId, clOrdID)
	reqDuration, ok := RequestDurations[key]
	if !ok {
		return
	}

	end := uint64(time.Now().UnixMicro())
	reqDuration.EndDuration = end
	reqDuration.Topic = topic

	go func(req RequestDuration) {
		RequestDurationHistogram.
			With(prometheus.Labels{"topic": req.Topic}).
			Observe(float64(req.EndDuration - req.StartDuration))
	}(reqDuration)

	// Release duration
	cleanUpDuration(key)
}

func ConsumedMetricCounter(topic, userId, clOrdId string) {
	IncomingCounter.With(prometheus.Labels{
		"topic": topic,
	}).Inc()

	StartRequestDuration(userId, clOrdId, RequestDuration{Topic: topic})
}

func PublishedMetricCounter(topic, userId, clOrdId string, success bool) {
	label := prometheus.Labels{"topic": topic}
	if success {
		SuccessCounter.With(label).Inc()
	} else {
		FailureCounter.With(label).Inc()
	}

	EndRequestDuration(userId, clOrdId, topic)
}
