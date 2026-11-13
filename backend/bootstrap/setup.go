// Package bootstrap contains additive, explicitly invoked first-install operations.
package bootstrap

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mtv2/backend/config"
	"mtv2/backend/utils"

	"github.com/elastic/go-elasticsearch/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//go:embed elasticsearch-index.json
var mapping string

func mongoClient(ctx context.Context) (*mongo.Client, error) {
	c, err := mongo.Connect(ctx, options.Client().ApplyURI(config.AppConfig.MongoDB.ConnectionString))
	if err != nil {
		return nil, err
	}
	if err = c.Ping(ctx, nil); err != nil {
		_ = c.Disconnect(ctx)
		return nil, err
	}
	return c, nil
}

// InitMongo never drops collections, changes documents, or replaces validators.
func InitMongo(ctx context.Context) error {
	c, err := mongoClient(ctx)
	if err != nil {
		return err
	}
	defer c.Disconnect(context.Background())
	cfg := config.AppConfig.MongoDB
	db := c.Database(cfg.DatabaseName)
	unique := func(keys bson.D) mongo.IndexModel {
		return mongo.IndexModel{Keys: keys, Options: options.Index().SetUnique(true)}
	}
	indexes := map[string][]mongo.IndexModel{
		cfg.Collections.Accounts:        {unique(bson.D{{Key: "username", Value: 1}}), unique(bson.D{{Key: "email", Value: 1}})},
		cfg.Collections.Invitations:     {unique(bson.D{{Key: "invite_code", Value: 1}}), {Keys: bson.D{{Key: "created_by", Value: 1}}}},
		cfg.Collections.ProcessedList:   {unique(bson.D{{Key: "user_id", Value: 1}, {Key: "task_id", Value: 1}}), {Keys: bson.D{{Key: "task_id", Value: 1}}}},
		cfg.Collections.Problems:        {unique(bson.D{{Key: "userId", Value: 1}, {Key: "taskId", Value: 1}}), {Keys: bson.D{{Key: "taskId", Value: 1}}}, {Keys: bson.D{{Key: "timestamp", Value: -1}}}, {Keys: bson.D{{Key: "uploader", Value: 1}}}},
		cfg.Collections.Comments:        {{Keys: bson.D{{Key: "userId", Value: 1}}}, {Keys: bson.D{{Key: "problemId", Value: 1}}}, {Keys: bson.D{{Key: "createTime", Value: 1}}}},
		cfg.Collections.AIUserSummaries: {unique(bson.D{{Key: "userId", Value: 1}})},
		cfg.Collections.WeeklyScores:    {unique(bson.D{{Key: "userId", Value: 1}, {Key: "year", Value: 1}, {Key: "weekId", Value: 1}}), {Keys: bson.D{{Key: "year", Value: 1}, {Key: "weekId", Value: 1}, {Key: "score", Value: -1}}}},
	}
	for collection, models := range indexes {
		if _, err := db.Collection(collection).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("ensure indexes for %s: %w", collection, err)
		}
	}
	return nil
}

// InitES preserves any existing index, including its mapping and documents.
func InitES(ctx context.Context) error {
	cfg := config.AppConfig.Elasticsearch
	// First-install container services use HTTP over loopback. Existing HTTPS services
	// are checked by the API using the configured CA, never initialized implicitly.
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: cfg.Hosts, Username: cfg.Username, Password: cfg.Password})
	if err != nil {
		return err
	}
	res, err := es.Indices.Exists([]string{cfg.IndexName}, es.Indices.Exists.WithContext(ctx))
	if err != nil {
		return err
	}
	status := res.StatusCode
	res.Body.Close()
	if status == http.StatusOK {
		fmt.Println("ES index exists; keeping existing mapping and data")
		return nil
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("check ES index: HTTP %d", status)
	}
	res, err = es.Indices.Create(cfg.IndexName, es.Indices.Create.WithBody(strings.NewReader(mapping)), es.Indices.Create.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("create ES index: HTTP %d", res.StatusCode)
	}
	return nil
}

func CreateAdmin(ctx context.Context, username, email, password string) error {
	if username == "" || email == "" || len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("admin requires username, email, and a password of 12-72 bytes")
	}
	c, err := mongoClient(ctx)
	if err != nil {
		return err
	}
	defer c.Disconnect(context.Background())
	accounts := c.Database(config.AppConfig.MongoDB.DatabaseName).Collection(config.AppConfig.MongoDB.Collections.Accounts)
	n, err := accounts.CountDocuments(ctx, bson.M{"$or": bson.A{bson.M{"username": username}, bson.M{"email": email}}})
	if err != nil {
		return err
	}
	if n != 0 {
		return fmt.Errorf("account already exists; no account changed")
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = accounts.InsertOne(ctx, bson.M{"username": username, "email": email, "password": hash, "is_admin": true, "points": 0, "token_version": 0, "created_at": time.Now().UTC()})
	return err
}
