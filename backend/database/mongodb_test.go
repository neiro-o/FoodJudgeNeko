package database

import (
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"mtv2/backend/config"
	"testing"
)

func TestStateCollectionsUseSelectedDatabase(t *testing.T) {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://127.0.0.1:27017"))
	if err != nil {
		t.Fatal(err)
	}
	config.AppConfig = &config.Config{}
	config.AppConfig.MongoDB.Collections.Accounts = "accounts"
	config.AppConfig.MongoDB.Collections.Invitations = "invitations"
	assignStateCollections(client.Database("shared"))
	assignLocalCollections(client.Database("local"))
	for _, c := range []*mongo.Collection{Accounts, Invitations, SearchQuotas, WeeklyScores, UserRankings} {
		if c.Database().Name() != "shared" {
			t.Fatal("state collection bound to wrong database")
		}
	}
	for _, c := range []*mongo.Collection{Problems, Comments, Notes, ProcessedList, AIUserSummaries} {
		if c.Database().Name() != "local" {
			t.Fatal("local collection bound to wrong database")
		}
	}
	if WeeklyScores.Name() != "weekly_scores" || UserRankings.Name() != "user_rankings" {
		t.Fatal("ranking fallback names incorrect")
	}
	config.AppConfig.MongoDB.Collections.WeeklyScores = "custom_weekly"
	config.AppConfig.MongoDB.Collections.UserRankings = "custom_rankings"
	assignStateCollections(client.Database("shared"))
	if WeeklyScores.Name() != "custom_weekly" || UserRankings.Name() != "custom_rankings" {
		t.Fatal("custom ranking names not preserved")
	}
	if SearchQuotas.Name() != "search_quotas" {
		t.Fatal("quota fallback name incorrect")
	}
}
