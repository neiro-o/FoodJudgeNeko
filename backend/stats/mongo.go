package stats

import (
	"context"
	"math"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoSource struct{ Comments, Malicious *mongo.Collection }

func (s MongoSource) MaliciousUserIDs(ctx context.Context) (map[string]bool, error) {
	cursor, err := s.Malicious.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"userId": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	ids := map[string]bool{}
	for cursor.Next(ctx) {
		var row bson.M
		if err = cursor.Decode(&row); err != nil {
			return nil, err
		}
		if id, ok := row["userId"].(string); ok {
			ids[id] = true
		}
	}
	return ids, cursor.Err()
}
func (s MongoSource) Scan(ctx context.Context, from, to int64, visit func(Comment) error) error {
	// Requiring the time index prevents an accidental 1.5M-document scan before
	// the explicit init-mongo migration has been run.
	cursor, err := s.Comments.Find(ctx, bson.M{"createTime": bson.M{"$gte": from, "$lt": to}, "content": bson.M{"$type": "string"}}, options.Find().SetHint(bson.D{{Key: "createTime", Value: 1}}).SetBatchSize(500).SetProjection(bson.M{"content": 1, "createTime": 1, "approveCount": 1, "userName": 1, "userId": 1, "isAnonymous": 1}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var row bson.M
		if err = cursor.Decode(&row); err != nil {
			return err
		}
		c, ok := decodeComment(row)
		if !ok {
			continue
		}
		if err = visit(c); err != nil {
			return err
		}
	}
	return cursor.Err()
}
func decodeComment(row bson.M) (Comment, bool) {
	id, ok := row["_id"].(primitive.ObjectID)
	if !ok {
		return Comment{}, false
	}
	content, ok := row["content"].(string)
	if !ok {
		return Comment{}, false
	}
	timestamp, ok := seconds(row["createTime"])
	if !ok {
		return Comment{}, false
	}
	user, _ := row["userId"].(string)
	name, _ := row["userName"].(string)
	// Missing anonymity metadata is handled conservatively.
	anonymous, known := row["isAnonymous"].(bool)
	if !known {
		anonymous = true
	}
	likes, _ := seconds(row["approveCount"])
	return Comment{ID: id.Hex(), Content: content, CreateTime: timestamp, Likes: likes, Name: name, IsAnonymous: anonymous, UserID: user}, true
}
func seconds(v any) (int64, bool) {
	switch x := v.(type) {
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case float64:
		if !math.IsNaN(x) && !math.IsInf(x, 0) && x >= 0 && x < math.MaxInt64 && x == math.Trunc(x) {
			return int64(x), true
		}
	}
	return 0, false
}
