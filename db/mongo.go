package db

import (
	"fmt"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
	"net/url"
)

func InitMongoDB() *mongo.Client {
	logger := logs.GetLogger()
	dbURL := fmt.Sprintf("mongodb+srv://%s:%s@%s/?retryWrites=true&w=majority",
		config.GetString("mongodb.user"),
		url.PathEscape(config.GetString("mongodb.password")),
		config.GetString("mongodb.host"))
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(dbURL).SetServerAPIOptions(serverAPI)

	client, err := mongo.Connect(opts)
	if err != nil {
		logger.Fatal("Error connecting to mongo client", zap.Error(err))
		return nil
	}

	logger.Info("You are successfully connected to MongoDB!")

	return client
}
