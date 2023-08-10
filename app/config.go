package app

import (
	"os"
	"path"

	"github.com/Undercurrent-Technologies/kprime-utilities/commons/log"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

// Config stores the application-wide configurations
var Config AppConfig
var logger = log.Logger

type AppConfig struct {
	HTTP              `yaml:"http"`
	Mongo             `yaml:"mongo"`
	Kafka             `yaml:"kafka"`
	Scheduler         `yaml:"scheduler"`
	NonceDiff         string `yaml:"nonce_diff" env:"NONCE_DIFF" env-default:"20"`
	MatchingEngineURL string `yaml:"matching_engine_url" env:"MATCHING_ENGINE_URL" env-default:"http://localhost:8080"`
}

type HTTP struct {
	NodeENV     string `yaml:"node_env" env:"NODE_ENV" env-default:"development"`
	ServerPort  string `yaml:"server_port" env:"SERVER_PORT" env-default:"8081"`
	MetricsPort string `yaml:"metrics_port" env:"METRICS_PORT" env-default:"2114"`
}

type Kafka struct {
	BrokerURL string `yaml:"broker_url" env:"BROKER_URL" env-default:"localhost:9092"`
}

type Mongo struct {
	Database string `yaml:"mongo_database" env:"MONGO_DATABASE" env-default:"option_exchange"`
	URL      string `yaml:"mongo_url" env:"MONGO_URL" env-default:"mongodb://localhost:27017"`
}

type Scheduler struct {
	MonitoringInterval string `yaml:"monitoring_interval" env:"MONITORING_INTERVAL" env-default:"1000"`
}

// LoadConfig loads configuration from the given list of paths and populates it into the Config variable.
// The configuration file(s) should be named as app.yaml.
// Environment variables with the prefix "RESTFUL_" in their names are also read automatically.
func LoadConfig() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err = godotenv.Load(path.Join(wd, ".env")); err != nil {
		logger.Info(".env not found, will use host environment variables")
	}

	if err = cleanenv.ReadEnv(&Config); err != nil {
		return err
	}

	logger.Infof("Environment: %v", Config.HTTP.NodeENV)
	logger.Infof("Server port: %v", Config.HTTP.ServerPort)
	logger.Infof("Metric port: %v", Config.HTTP.MetricsPort)
	logger.Infof("MongoDB url: %v", Config.Mongo.URL)
	logger.Infof("Kafka url: %v", Config.Kafka.BrokerURL)

	return nil
}
