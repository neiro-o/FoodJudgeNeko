package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// XOR + base64 keeps the URL template out of plain-text source searches.
// This is obfuscation, not a secret or an access-control mechanism.
const problemLinkTemplate = "OycnIyBpfHwpIid9PjY6JyYyPX0wPD58KzoyPD42OnwgJzInOjB8Oj0xPCt+JzIgOH43NicyOj9sMTopGjdudTA7Mj09Nj9uPCM2PSQ2MSU6NiR1NSE8PgMyNDZuOj0xPCt+PjYgIDI0NnU0Mj42BzIgOBo3bnU/PDQ6PW51PCE6NDo9EDsyPT02P251JzIhNDYnATYjPyoaN252IHUlPCc2BzIgOB08bnYgdSs+NjogIG4="

func GetProblemLink(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	objectID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		utils.BadRequestResponse(c, "Invalid MongoID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	var item bson.Raw
	err = database.Problems.FindOne(ctx, bson.M{"_id": objectID},
		options.FindOne().SetProjection(bson.M{
			"detail.taskInfo.voteTaskNo":    1,
			"comment.pageContent.commentId": 1,
		}),
	).Decode(&item)
	if err == mongo.ErrNoDocuments {
		utils.NotFoundResponse(c, "Problem not found")
		return
	}
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to look up problem")
		return
	}
	link, err := buildProblemLink(item)
	if err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}
	utils.SuccessResponse(c, link)
}

func problemLinkID(value bson.RawValue) string {
	switch value.Type {
	case bson.TypeString:
		return value.StringValue()
	case bson.TypeInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case bson.TypeInt32:
		return strconv.FormatInt(int64(value.Int32()), 10)
	default:
		return ""
	}
}

func buildProblemLink(item bson.Raw) (string, error) {
	rawProblemID := problemLinkID(item.Lookup("detail", "taskInfo", "voteTaskNo"))
	if strings.TrimSpace(rawProblemID) == "" {
		return "", fmt.Errorf("Problem is missing detail.taskInfo.voteTaskNo")
	}
	firstReplyID := problemLinkID(item.Lookup("comment", "0", "pageContent", "0", "commentId"))
	template, err := base64.StdEncoding.DecodeString(problemLinkTemplate)
	if err != nil {
		return "", fmt.Errorf("Invalid problem link template")
	}
	for i := range template {
		template[i] ^= 0x53
	}
	return fmt.Sprintf(string(template), url.QueryEscape(firstReplyID), url.QueryEscape(rawProblemID)), nil
}
