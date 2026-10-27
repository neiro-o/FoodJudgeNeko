package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"mtv2/backend/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Client          *mongo.Client
	StateClient     *mongo.Client
	StateDB         *mongo.Database
	DB              *mongo.Database
	Accounts        *mongo.Collection
	Invitations     *mongo.Collection
	ProcessedList   *mongo.Collection
	Problems        *mongo.Collection
	UploadHistory   *mongo.Collection
	Comments        *mongo.Collection
	Malicious       *mongo.Collection
	Notes           *mongo.Collection
	UserRankings    *mongo.Collection
	AIUserSummaries *mongo.Collection
	WeeklyScores    *mongo.Collection
	SearchQuotas    *mongo.Collection
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
	StateDB = DB
	if config.AppConfig.SharedState.URI != "" {
		state, e := mongo.Connect(ctx, options.Client().ApplyURI(config.AppConfig.SharedState.URI))
		if e != nil {
			client.Disconnect(ctx)
			return fmt.Errorf("failed to connect state MongoDB: %w", e)
		}
		if e = state.Ping(ctx, nil); e != nil {
			state.Disconnect(ctx)
			client.Disconnect(ctx)
			return fmt.Errorf("failed to ping state MongoDB: %w", e)
		}
		StateClient = state
		name := config.AppConfig.SharedState.Database
		if name == "" {
			name = config.AppConfig.MongoDB.DatabaseName
		}
		StateDB = state.Database(name)
	}
	assignStateCollections(StateDB)
	assignLocalCollections(DB)

	// Ensure the AI summary cache has a unique index on userId so a
	// concurrent generation can never leave two cache rows for the same
	// user (the handler upserts by userId).
	if os.Getenv("MTV2_SKIP_STARTUP_INDEXES") != "true" {
		if _, err := AIUserSummaries.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(true),
		}); err != nil {
			fmt.Printf("Warning: failed to create ai_user_summaries index: %v\n", err)
		}

	}
	fmt.Println("Connected to MongoDB successfully")
	return nil
}

func Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if StateClient != nil {
		if err := StateClient.Disconnect(ctx); err != nil {
			return err
		}
	}
	if Client != nil {
		return Client.Disconnect(ctx)
	}
	return nil
}

func assignStateCollections(db *mongo.Database) {
	Accounts = db.Collection(config.AppConfig.MongoDB.Collections.Accounts)
	Invitations = db.Collection(config.AppConfig.MongoDB.Collections.Invitations)
	name := config.AppConfig.MongoDB.Collections.SearchQuotas
	if name == "" {
		name = "search_quotas"
	}
	SearchQuotas = db.Collection(name)
	name = config.AppConfig.MongoDB.Collections.UserRankings
	if name == "" {
		name = "user_rankings"
	}
	UserRankings = db.Collection(name)
	name = config.AppConfig.MongoDB.Collections.WeeklyScores
	if name == "" {
		name = "weekly_scores"
	}
	WeeklyScores = db.Collection(name)
}

func assignLocalCollections(db *mongo.Database) {
	ProcessedList = db.Collection(config.AppConfig.MongoDB.Collections.ProcessedList)
	Problems = db.Collection(config.AppConfig.MongoDB.Collections.Problems)
	UploadHistory = db.Collection("upload_history")
	Comments = db.Collection(config.AppConfig.MongoDB.Collections.Comments)
	Malicious = db.Collection("malicious")
	Notes = db.Collection(config.AppConfig.MongoDB.Collections.Notes)

	aiUserSummariesCollectionName := config.AppConfig.MongoDB.Collections.AIUserSummaries
	if aiUserSummariesCollectionName == "" {
		aiUserSummariesCollectionName = "ai_user_summaries"
	}
	AIUserSummaries = db.Collection(aiUserSummariesCollectionName)

}
