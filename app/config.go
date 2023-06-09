package app

import (
	"path"
	"runtime"

	log "git.devucc.name/dependencies/utilities/commons/log"
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
	NodeENV string `yaml:"node_env" env:"NODE_ENV" env-default:"development"`
	Port    string `yaml:"port" env:"PORT" env-default:"8081"`
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
	_, b, _, _ := runtime.Caller(0)
	rootDir := path.Join(b, "../../")
	err := godotenv.Load(path.Join(rootDir, ".env"))
	if err != nil {
		return err
	}

	err = cleanenv.ReadEnv(&Config)
	if err != nil {
		return err
	}

	logger.Infof("Environment: %v", Config.HTTP.NodeENV)
	logger.Infof("Server port: %v", Config.HTTP.Port)
	logger.Infof("MongoDB url: %v", Config.Mongo.URL)
	logger.Infof("Kafka url: %v", Config.Kafka.BrokerURL)

	return nil
}
