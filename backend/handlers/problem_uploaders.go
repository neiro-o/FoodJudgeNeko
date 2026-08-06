package handlers

import (
	"context"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type problemUploaderFields struct {
	Uploader          string   `bson:"uploader"`
	PreviousUploaders []string `bson:"previous_uploaders"`
}

type problemUploadersResponse struct {
	Uploader              string   `json:"uploader"`
	UploaderName          string   `json:"uploader_name"`
	PreviousUploaders     []string `json:"previous_uploaders"`
	PreviousUploaderNames []string `json:"previous_uploaders_name"`
}

// GetProblemUploaders returns the current and previous uploaders for a problem,
// together with account usernames aligned to the corresponding uploader IDs.
func GetProblemUploaders(c *gin.Context) {
	mongoID := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(mongoID)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid MongoID")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var problem problemUploaderFields
	err = database.Problems.FindOne(
		ctx,
		bson.M{"_id": objectID},
		options.FindOne().SetProjection(bson.M{
			"_id":                0,
			"uploader":           1,
			"previous_uploaders": 1,
		}),
	).Decode(&problem)
	if err == mongo.ErrNoDocuments {
		utils.NotFoundResponse(c, "Problem not found")
		return
	}
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to look up problem uploaders")
		return
	}

	if problem.PreviousUploaders == nil {
		problem.PreviousUploaders = []string{}
	}

	usernames, err := findAccountUsernames(ctx, append([]string{problem.Uploader}, problem.PreviousUploaders...))
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to look up uploader accounts")
		return
	}

	previousUploaderNames := make([]string, len(problem.PreviousUploaders))
	for i, uploaderID := range problem.PreviousUploaders {
		previousUploaderNames[i] = usernames[uploaderID]
	}

	utils.SuccessResponse(c, problemUploadersResponse{
		Uploader:              problem.Uploader,
		UploaderName:          usernames[problem.Uploader],
		PreviousUploaders:     problem.PreviousUploaders,
		PreviousUploaderNames: previousUploaderNames,
	})
}

func findAccountUsernames(ctx context.Context, uploaderIDs []string) (map[string]string, error) {
	usernames := make(map[string]string, len(uploaderIDs))
	objectIDs := make([]primitive.ObjectID, 0, len(uploaderIDs))
	seen := make(map[primitive.ObjectID]struct{}, len(uploaderIDs))

	for _, uploaderID := range uploaderIDs {
		usernames[uploaderID] = ""
		objectID, err := primitive.ObjectIDFromHex(uploaderID)
		if err != nil {
			continue
		}
		if _, exists := seen[objectID]; exists {
			continue
		}
		seen[objectID] = struct{}{}
		objectIDs = append(objectIDs, objectID)
	}

	if len(objectIDs) == 0 {
		return usernames, nil
	}

	cursor, err := database.Accounts.Find(
		ctx,
		bson.M{"_id": bson.M{"$in": objectIDs}},
		options.Find().SetProjection(bson.M{"username": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var account struct {
			ID       primitive.ObjectID `bson:"_id"`
			Username string             `bson:"username"`
		}
		if err := cursor.Decode(&account); err != nil {
			return nil, err
		}
		usernames[account.ID.Hex()] = account.Username
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return usernames, nil
}
