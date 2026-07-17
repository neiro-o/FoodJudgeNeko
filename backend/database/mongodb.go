package database

import (
	"context"
	"fmt"
	"time"

	"mtv2/backend/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Client          *mongo.Client
	DB              *mongo.Database
	Accounts        *mongo.Collection
	Invitations     *mongo.Collection
	ProcessedList   *mongo.Collection
	Problems        *mongo.Collection
	Comments        *mongo.Collection
	Malicious       *mongo.Collection
	Notes           *mongo.Collection
	UserRankings    *mongo.Collection
	AIUserSummaries *mongo.Collection
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
	Notes = DB.Collection(config.AppConfig.MongoDB.Collections.Notes)
	userRankingsCollectionName := config.AppConfig.MongoDB.Collections.UserRankings
	if userRankingsCollectionName == "" {
		userRankingsCollectionName = "user_rankings"
	}
	UserRankings = DB.Collection(userRankingsCollectionName)

	aiUserSummariesCollectionName := config.AppConfig.MongoDB.Collections.AIUserSummaries
	if aiUserSummariesCollectionName == "" {
		aiUserSummariesCollectionName = "ai_user_summaries"
	}
	AIUserSummaries = DB.Collection(aiUserSummariesCollectionName)

	// Ensure the AI summary cache has a unique index on userId so a
	// concurrent generation can never leave two cache rows for the same
	// user (the handler upserts by userId).
	if _, err := AIUserSummaries.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userId", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		fmt.Printf("Warning: failed to create ai_user_summaries index: %v\n", err)
	}

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
