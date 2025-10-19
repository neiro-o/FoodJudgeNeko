package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type InviteCode struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	InviteCode string             `bson:"invite_code" json:"invite_code"`
	CreateTime time.Time          `bson:"create_time" json:"create_time"`
	ExpireTime time.Time          `bson:"expire_time" json:"expire_time"`
	Used       bool               `bson:"used" json:"used"`
}
