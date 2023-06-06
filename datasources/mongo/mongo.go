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

type MongoDB struct {
	Client *mongo.Client
}

var Database *MongoDB
var logger = log.Logger

func InitConnection(uri string) error {
	logger.Infof("Database connecting...")

	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return err
	}

	if err := client.Ping(context.Background(), readpref.Primary()); err != nil {
		return err
	}

	logger.Infof("Database connected!")

	Database = &MongoDB{Client: client}

	return nil
}

func (db *MongoDB) InitCollection(collectionName string) *mongo.Collection {
	return db.Client.Database(app.Config.Mongo.Database).Collection(collectionName)
}
