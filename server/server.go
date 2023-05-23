package server

import (
	"fmt"
	"log"
	"net/http"
	"pickup/app"

	"github.com/gorilla/mux"
)

func Start() {
	// Initialize ENV
	err := app.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load ENV")
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
