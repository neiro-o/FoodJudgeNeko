package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Reply represents a reply in the conversation
type Reply struct {
	Role      string `json:"role"`
	Timestamp int64  `json:"timestamp"`
	Content   string `json:"content"`
}

// Appeal represents an appeal with evidence
type Appeal struct {
	Role      string   `json:"role"`
	Timestamp int64    `json:"timestamp"`
	Content   string   `json:"content"`
	Pics      []string `json:"pics"`
}

// Order represents an order item
type Order struct {
	Name      string   `json:"name"`
	Count     int      `json:"count"`
	Desc      string   `json:"desc"`
	Selection []string `json:"selection"`
	Pic       string   `json:"pic"`
	Others    string   `json:"others"`
}

// OrderDetail represents detailed order information
type OrderDetail struct {
	OrderStarted   int64  `json:"order_started"`
	OrderFinished  int64  `json:"order_finished"`
	DeliverTime    int    `json:"deliver_time"`
	TotalTime      int    `json:"total_time"`
	DeliverBy      string `json:"deliver_by"`
	Note           string `json:"note"`
	Tableware      int    `json:"tableware"`
	Invoice        bool   `json:"invoice"`
}

// Comment represents a comment on the problem
type Comment struct {
	UserID    int    `json:"userid"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Choice    int    `json:"choice"`
}

// Problem represents the main problem structure stored in ES
type Problem struct {
	ID           primitive.ObjectID `json:"id"`
	ProblemType  int                `json:"problem_type"` // 1: takeout, 2: dine-in, 3: takeout-refund, 4: dinein-refund, 5: other
	UserReview   string             `json:"user_review,omitempty"`
	ReviewPics   []string           `json:"review_pics"`
	Timestamp    int64              `json:"timestamp"`
	Replies      []Reply            `json:"replies"`
	Appeals      []Appeal           `json:"appeals"`
	OrderInfo    string             `json:"order_info"`
	Orders       []Order            `json:"orders,omitempty"`
	OrderDetail  *OrderDetail       `json:"order_detail,omitempty"`
	Others       string             `json:"others,omitempty"`
	Answer       int                `json:"answer"`
	Ratio1       int                `json:"ratio_1"`
	Ratio2       int                `json:"ratio_2"`
	Comments     []Comment          `json:"comments"`
	CreatedAt    time.Time          `json:"created_at"`
	Uploader     string             `json:"uploader"`
	TaskID       string             `json:"taskId"`
	MtUserID     int                `json:"mtUserId"`
}

// RawProblem represents the raw JSON data stored in MongoDB
type RawProblem struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RawData   interface{}        `bson:"raw_data" json:"raw_data"`
	Processed bool               `bson:"processed" json:"processed"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// SearchProblemsRequest represents simplified search parameters
type SearchProblemsRequest struct {
	Query    string `json:"query" binding:"required"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// ParsedLink represents a parsed link with user and task information
type ParsedLink struct {
	MtUserID int    `json:"mtuserid" binding:"required"`
	TaskID   string `json:"task_id" binding:"required"`
}

// AddProblemsRequest represents request to add parsed links
type AddProblemsRequest struct {
	ParsedLinks []ParsedLink `json:"parsed_links" binding:"required"`
}

// QueueItem represents an item in the processing queue
type QueueItem struct {
	UserID     primitive.ObjectID `json:"user_id"`
	MtUserID   int                `json:"mtuserid"`
	TaskID     string             `json:"task_id"`
	UploadTime time.Time          `json:"upload_time"`
	UploadIP   string             `json:"upload_ip"`
}

// ProblemDetailResponse represents detailed problem response
type ProblemDetailResponse struct {
	Problem *Problem `json:"problem"`
}

// ProblemListResponse represents paginated problem list response
type ProblemListResponse struct {
	Problems []Problem `json:"problems"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// SearchResult represents a single search result item
type SearchResult struct {
	ID           string  `json:"id"`
	Review       string  `json:"review"`
	GenerateTime string  `json:"generate_time"`
	Uploader     string  `json:"uploader"`
	Answer       int     `json:"answer"`
	Ratio1       int     `json:"ratio_1"`
	Ratio2       int     `json:"ratio_2"`
	MatchScore   float64 `json:"match_score"`
}

// SearchProblemsResponse represents the search results response
type SearchProblemsResponse struct {
	Results  []SearchResult `json:"results"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}
