package mongo

import (
	"context"
	"pickup/app"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"git.devucc.name/dependencies/utilities/commons/log"
)

type Database struct {
	Client *mongo.Client
}

var logger = log.Logger

func InitConnection(uri string) (*Database, error) {
	logger.Infof("Database connecting...")

	credential := options.Credential{
		Username: app.Config.Mongo.User,
		Password: app.Config.Mongo.Password,
	}

	client, err := mongo.NewClient(options.Client().ApplyURI(uri).SetAuth(credential))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)

	defer cancel()

	if err != nil {
		return nil, err
	}

	err = client.Ping(context.Background(), readpref.Primary())
	if err != nil {
		return nil, err
	}

	logger.Infof("Database connected!")

	return &Database{client}, nil
}

func (db *Database) InitCollection(collectionName string) *mongo.Collection {
	return db.Client.Database(app.Config.Mongo.Database).Collection(collectionName)
}
