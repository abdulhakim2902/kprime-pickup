package server

import (
	"fmt"
	"log"
	"net/http"
	"pickup/app"
	"pickup/datasources/kafka"
	"pickup/datasources/mongo"
	"pickup/service"

	utilitiesLog "git.devucc.name/dependencies/utilities/commons/log"
	"git.devucc.name/dependencies/utilities/repository/mongodb"
)

var logger = utilitiesLog.Logger
var topics = "ENGINE,CANCELLED_ORDER,ENGINE_SAVED"

func Start() {
	// Initialize ENV
	err := app.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load ENV")
	}

	// Connect Database
	db, err := mongo.InitConnection(app.Config.Mongo.URL)
	if err != nil {
		log.Fatal("Failed to initialize database!", err)
	}

	// Initialize Consumer
	k, err := kafka.InitConnection(app.Config.Kafka.BrokerURL, topics)
	if err != nil {
		log.Fatal("Failed to initialize kafka connection!", err)
	}

	// Initialize Repository
	or := mongodb.NewOrderRepository(db)
	tr := mongodb.NewTradeRepository(db)
	ar := mongodb.NewActivityRepository(db)

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
		log.Fatal(err)
	}
}
