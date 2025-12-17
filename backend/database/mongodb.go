package database

import (
	"context"
	"fmt"
	"time"

	"mtv2/backend/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Client        *mongo.Client
	DB            *mongo.Database
	Accounts      *mongo.Collection
	Invitations   *mongo.Collection
	ProcessedList *mongo.Collection
	Problems      *mongo.Collection
	Comments      *mongo.Collection
	Malicious     *mongo.Collection
)

func Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(config.AppConfig.MongoDB.ConnectionString)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	Client = client
	DB = client.Database(config.AppConfig.MongoDB.DatabaseName)
	Accounts = DB.Collection(config.AppConfig.MongoDB.Collections.Accounts)
	Invitations = DB.Collection(config.AppConfig.MongoDB.Collections.Invitations)
	ProcessedList = DB.Collection(config.AppConfig.MongoDB.Collections.ProcessedList)
	Problems = DB.Collection(config.AppConfig.MongoDB.Collections.Problems)
	Comments = DB.Collection(config.AppConfig.MongoDB.Collections.Comments)
	Malicious = DB.Collection("malicious")

	fmt.Println("Connected to MongoDB successfully")
	return nil
}

func Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if Client != nil {
		return Client.Disconnect(ctx)
	}
	return nil
}
