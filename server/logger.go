package server

import (
	"errors"
	"os"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/logs"
)

const PICKUP logs.LoggerType = "PICKUP"

func initLogger() error {
	// Initialize Logger
	logs.InitLogger(PICKUP)

	// Logger With Papertrail
	if os.Getenv("LOG_WITH_PAPERTRAIL") == "true" {
		if os.Getenv("PAPERTRAIL_HOST") == "" {
			return errors.New("PAPERTRAIL_HOST is not set")
		}

		if os.Getenv("PAPERTRAIL_PORT") == "" {
			return errors.New("PAPERTRAIL_PORT is not set")
		}

		logs.WithPaperTrail()
	}

	// Enable discord logs if DISLOG_WEBHOOK_URL is provided
	if len(os.Getenv("DISLOG_WEBHOOK_URL")) > 0 {
		logs.WithDiscord(os.Getenv("DISLOG_WEBHOOK_URL"))
	}

	return nil
}
