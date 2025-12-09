package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Account struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username          string             `bson:"username" json:"username"`
	Password          string             `bson:"password" json:"-"` // Exclude from JSON
	Email             string             `bson:"email" json:"email"`
	Points            int                `bson:"points" json:"points"`
	IsAdmin           bool               `bson:"is_admin" json:"is_admin"`
	TokenVersion      int                `bson:"token_version" json:"-"`                 // Token version for invalidating old tokens
	PasswordChangedAt time.Time          `bson:"password_changed_at,omitempty" json:"-"` // Track password changes
	CreatedAt         time.Time          `bson:"created_at" json:"created_at"`
}
