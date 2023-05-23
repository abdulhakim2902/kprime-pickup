package server

import (
	"fmt"
	"log"
	"net/http"
	"pickup/app"
	"pickup/datasources/kafka"
	"pickup/datasources/mongo"

	utilitiesLog "git.devucc.name/dependencies/utilities/commons/log"
	"github.com/gorilla/mux"
)

var logger = utilitiesLog.Logger
var topics = "NEW_ORDER,ENGINE,ORDERBOOK,CANCELLED_ORDER"

func Start() {
	// Initialize ENV
	err := app.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load ENV")
	}

	// Connect Database
	_, err = mongo.InitConnection(app.Config.Mongo.URL)
	if err != nil {
		log.Fatal("Failed to initialize database!")
	}

	// Initialize Consumer
	_, err = kafka.InitConnection(app.Config.Kafka.BrokerURL, topics)
	if err != nil {
		log.Fatal("Failed to initialize kafka connection!", err)
	}

	router := mux.NewRouter()
	run(router)
}

func run(router *mux.Router) {
	port := fmt.Sprintf(":%v", app.Config.HTTP.Port)
	log.Printf("Server %v is running on localhost:%v\n", app.Version, app.Config.HTTP.Port)
	err := http.ListenAndServe(port, router)
	if err != nil {
		log.Fatal(err)
	}
}
