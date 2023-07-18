package server

import (
	"fmt"
	"log"
	"net/http"
	"pickup/app"
	"pickup/datasources/collector"
	"pickup/datasources/kafka"
	"pickup/datasources/mongo"
	"pickup/service"
	"strconv"
	"time"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/metrics"
	"github.com/Undercurrent-Technologies/kprime-utilities/types"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
	"github.com/Undercurrent-Technologies/kprime-utilities/repository/mongodb"
	"github.com/go-co-op/gocron"
)

var topics = []types.Topic{
	types.ENGINE,
	types.CANCELLED_ORDER,
	types.ENGINE_SAVED,
	types.CANCELLED_ORDER_SAVED,
}

func Start() {
	// Initialize ENV
	if err := app.LoadConfig(); err != nil {
		logs.Log.Fatal().Err(err).Msg("Failed to load ENV!")
	}

	// Initialize Logger
	if err := initLogger(); err != nil {
		logs.Log.Fatal().Err(err).Msg("Failed to initialize logger")
	}

	// Connect Database
	if err := mongo.InitConnection(app.Config.Mongo.URL); err != nil {
		logs.Log.Fatal().Err(err).Msg("Failed to connect database!")
	}

	// Initialize Consumer
	k, err := kafka.InitConnection(app.Config.Kafka.BrokerURL, topics...)
	if err != nil {
		logs.Log.Fatal().Err(err).Msg("Failed to connect kafka!")
	}

	// Initialize Scheduler
	loc, err := time.LoadLocation("Asia/Singapore")
	if err != nil {
		loc = time.UTC
	}

	i, err := strconv.Atoi(app.Config.MonitoringInterval)
	if err != nil {
		i = 1000
	}

	s := gocron.NewScheduler(loc)

	// Initialize MongoDB Repository
	r := mongodb.NewRepositories(mongo.Database)

	// Initialize Service
	ms := service.NewManagerService(k, r)
	js := service.NewJobService(r)

	// Register scheduler
	s.Every(i).Milliseconds().Do(js.NonceMonitoring)

	// Start scheduler
	s.StartAsync()

	// Subscribe to kafka
	k.Subscribe(ms.HandlePickup)

	// Close kafka connection
	k.CloseConnection()

	// Run server
	serveMetric()
	run()
}

func run() {
	port := fmt.Sprintf(":%v", app.Config.HTTP.ServerPort)
	log.Printf("Server %v is running on localhost:%v\n", app.Version, app.Config.HTTP.ServerPort)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		logs.Log.Fatal().Err(err).Msg(fmt.Sprintf("Failed to listen and serve on port %s", app.Config.HTTP.ServerPort))
	}
}

func serveMetric() {
	go func() {
		m := metrics.NewMetrics()
		m.RegisterCollector(
			collector.IncomingCounter,
			collector.SuccessCounter,
			collector.RequestDurationHistogram,
		)

		if err := m.Serve(); err != nil {
			logs.Log.Fatal().Err(err).Msg(fmt.Sprintf("Failed to listen and serve on port%s!", app.Config.MetricsPort))
		}
	}()
}
