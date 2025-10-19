package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LLMAnswerRequest struct {
	ProblemID string `json:"problem_id" binding:"required"`
	Question  string `json:"question" binding:"required"`
}

type LLMAnswerResponse struct {
	ProblemID string    `json:"problem_id"`
	Answer    string    `json:"answer"`
	CreatedAt time.Time `json:"created_at"`
}

type AIAnswer struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProblemID primitive.ObjectID `bson:"problem_id" json:"problem_id"`
	Answer    string             `bson:"answer" json:"answer"`
	Model     string             `bson:"model" json:"model"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}
