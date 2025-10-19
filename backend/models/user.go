package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Username     string              `bson:"username" json:"username"`
	PasswordHash string              `bson:"password_hash" json:"-"`
	Email        string              `bson:"email" json:"email"`
	Role         string              `bson:"role" json:"role"` // admin, user, etc.
	InvitedBy    *primitive.ObjectID `bson:"invited_by,omitempty" json:"invited_by,omitempty"`
	CreatedAt    time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time           `bson:"updated_at" json:"updated_at"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username   string `json:"username" binding:"required,min=3,max=50"`
	Password   string `json:"password" binding:"required,min=6"`
	Email      string `json:"email" binding:"required,email"`
	InviteCode string `json:"invite_code" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type LoginResponse struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	ExpiresAt int64  `json:"expires_at"`
}
