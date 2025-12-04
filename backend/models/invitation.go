package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Invitation struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	InviteCode string            `bson:"invite_code" json:"invite_code"`
	CreatedBy  primitive.ObjectID `bson:"created_by" json:"created_by"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
	Used       bool               `bson:"used" json:"used"`
	UsedBy     *primitive.ObjectID `bson:"used_by,omitempty" json:"used_by,omitempty"`
}

