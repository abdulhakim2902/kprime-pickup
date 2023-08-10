package collector

import (
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

type RequestDurations struct {
	RequestDurations map[string]RequestDuration
	Mutex            *sync.Mutex
}

func (r *RequestDurations) StartRequestDuration(t, k string) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	r.consumedMetricCounter(t)

	rd := RequestDuration{
		Topic:         t,
		StartDuration: uint64(time.Now().UnixMicro()),
	}

	r.RequestDurations[k] = rd
}

func (r *RequestDurations) EndRequestDuration(t, k string, v bool) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	r.publishedMetricCounter(t, v)

	reqDuration, ok := r.RequestDurations[k]
	if !ok {
		return
	}

	reqDuration.EndDuration = uint64(time.Now().UnixMicro())
	reqDuration.Topic = t

	go func(req RequestDuration) {
		RequestDurationHistogram.
			With(prometheus.Labels{"topic": req.Topic}).
			Observe(float64(req.EndDuration - req.StartDuration))
	}(reqDuration)

	delete(r.RequestDurations, k)
}

func (r *RequestDurations) consumedMetricCounter(topic string) {
	IncomingCounter.With(prometheus.Labels{
		"topic": topic,
	}).Inc()
}

func (r *RequestDurations) publishedMetricCounter(topic string, success bool) {
	label := prometheus.Labels{"topic": topic}
	if success {
		SuccessCounter.With(label).Inc()
	} else {
		FailureCounter.With(label).Inc()
	}
}
