package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"mt-backend/config"
	"mt-backend/models"
)

// GenerateSessionID creates a cryptographically secure random session ID
func GenerateSessionID() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CreateSession generates a new session and stores it in both MongoDB and Redis
func CreateSession(userID primitive.ObjectID, ipAddress, userAgent string, maxAge int) (*models.Session, error) {
	sessionID, err := GenerateSessionID()
	if err != nil {
		return nil, err
	}

	session := &models.Session{
		SessionID: sessionID,
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Duration(maxAge) * time.Second),
		CreatedAt: time.Now(),
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	// Store in MongoDB for persistent session management
	ctx := context.Background()
	collection := config.MongoDB.Collection("sessions")
	_, err = collection.InsertOne(ctx, session)
	if err != nil {
		return nil, err
	}

	// Store in Redis for fast validation with expiration
	err = StoreSessionInRedis(sessionID, userID.Hex(), maxAge)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// StoreSessionInRedis stores session ID with user ID in Redis with expiration
func StoreSessionInRedis(sessionID, userID string, maxAge int) error {
	ctx := context.Background()
	key := "session:" + sessionID
	return config.RD.Set(ctx, key, userID, time.Duration(maxAge)*time.Second).Err()
}

// ValidateSession checks if session exists and is valid in Redis first, then MongoDB
func ValidateSession(sessionID string) (primitive.ObjectID, bool) {
	ctx := context.Background()
	
	// Check Redis first for performance
	key := "session:" + sessionID
	userID, err := config.RD.Get(ctx, key).Result()
	if err == nil && userID != "" {
		objectID, err := primitive.ObjectIDFromHex(userID)
		if err == nil {
			return objectID, true
		}
	}

	// Fallback to MongoDB if Redis miss or error
	collection := config.MongoDB.Collection("sessions")
	var session models.Session
	err = collection.FindOne(ctx, bson.M{
		"session_id": sessionID,
		"expires_at": bson.M{"$gt": time.Now()},
	}).Decode(&session)

	if err != nil {
		return primitive.NilObjectID, false
	}

	// Refresh Redis cache
	StoreSessionInRedis(sessionID, session.UserID.Hex(), 86400*7)
	
	return session.UserID, true
}

// InvalidateSession removes session from both Redis and MongoDB
func InvalidateSession(sessionID string) error {
	ctx := context.Background()
	
	// Remove from Redis
	key := "session:" + sessionID
	config.RD.Del(ctx, key)

	// Remove from MongoDB
	collection := config.MongoDB.Collection("sessions")
	_, err := collection.DeleteOne(ctx, bson.M{"session_id": sessionID})
	return err
}

// CleanupExpiredSessions removes expired sessions from MongoDB (can be run periodically)
func CleanupExpiredSessions() error {
	ctx := context.Background()
	collection := config.MongoDB.Collection("sessions")
	_, err := collection.DeleteMany(ctx, bson.M{
		"expires_at": bson.M{"$lt": time.Now()},
	})
	return err
}
