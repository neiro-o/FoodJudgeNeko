package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WeeklyScore mirrors a document in the mtv2.weekly_scores collection.
type WeeklyScore struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"userId" json:"userId"`
	Year      int                `bson:"year" json:"year"`
	WeekID    int                `bson:"weekId" json:"weekId"`
	Score     float64            `bson:"score" json:"score"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}
