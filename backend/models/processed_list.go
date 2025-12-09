package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProcessedListItem struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"user_id" json:"user_id"`
	TaskID    string             `bson:"task_id" json:"task_id"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

