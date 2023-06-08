package server

import (
	"fmt"
	"log"
	"net/http"
	"pickup/app"
	"pickup/datasources/kafka"
	"pickup/datasources/mongo"
	"pickup/service"

	"git.devucc.name/dependencies/utilities/commons/logs"
	"git.devucc.name/dependencies/utilities/repository/mongodb"
)

const PICKUP logs.LoggerType = "PICKUP"

var topics = "ENGINE,CANCELLED_ORDER,ENGINE_SAVED"

func Start() {
	// Logger
	logs.InitLogger(PICKUP)

	// Initialize ENV
	if err := app.LoadConfig(); err != nil {
		logs.Log.Fatal().Err(err).Msg("Failed to load ENV!")
	}

	// Connect Database
	if err := mongo.InitConnection(app.Config.Mongo.URL); err != nil {
		logs.Log.Fatal().Err(err).Msg("Failed to connect database!")
	}

	// Initialize Consumer
	k, err := kafka.InitConnection(app.Config.Kafka.BrokerURL, topics)
	if err != nil {
		logs.Log.Fatal().Err(err).Msg("Failed to connect kafka!")
	}

	// Initialize MongoDB Repository
	or := mongodb.NewOrderRepository(mongo.Database)
	tr := mongodb.NewTradeRepository(mongo.Database)
	ar := mongodb.NewActivityRepository(mongo.Database)

	// Initialize Service
	ms := service.NewManagerService(k, ar, or, tr)

	// Subscribe to kafka
	k.Subscribe(ms.HandlePickup)

	// Close kafka connection
	k.CloseConnection()

	// Run server
	run()
}

func run() {
	port := fmt.Sprintf(":%v", app.Config.HTTP.Port)
	log.Printf("Server %v is running on localhost:%v\n", app.Version, app.Config.HTTP.Port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		logs.Log.Fatal().Err(err).Msg(fmt.Sprintf("Failed to listen and serve on port %s", app.Config.HTTP.Port))
	}
}
