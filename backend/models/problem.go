package models

// Elasticsearch document models for problem/review evaluation records
// Based on ES_Params.md specification

// Reply represents a reply in the replies array
type Reply struct {
	Role      string `json:"role"`      // merchant, user, or others_X
	Timestamp int64  `json:"timestamp"` // timestamp in seconds
	Content   string `json:"content"`   // reply content
}

// Appeal represents an appeal in the appeals array
type Appeal struct {
	Role      string   `json:"role"`      // merchant, user, or others_X
	Timestamp int64    `json:"timestamp"` // appeal submission timestamp in seconds
	Content   string   `json:"content"`   // appeal text description
	Pics      []string `json:"pics"`      // array of evidence image URLs
}

// Order represents an order item in the orders array
type Order struct {
	Name      string   `json:"name"`      // product name
	Count     int      `json:"count"`     // purchase quantity
	Desc      string   `json:"desc"`      // product description
	Selection []string `json:"selection"` // user-selected specifications or flavors
	Pic       string   `json:"pic"`       // product image URL
	Others    string   `json:"others"`    // other additional information or notes
}

// OrderDetail represents order detail information for delivery scenarios
type OrderDetail struct {
	OrderStarted  int64  `json:"order_started"`  // order placement timestamp (seconds)
	OrderFinished int64  `json:"order_finished"` // completion timestamp (seconds), 0 for pickup
	DeliverTime   int64  `json:"deliver_time"`   // delivery time in seconds, -1 for unavailable
	TotalTime     int64  `json:"total_time"`     // total time from order to completion in seconds
	DeliverBy     string `json:"deliver_by"`     // delivery party: merchant, meituan, or user
	Note          string `json:"note"`           // user notes
	Utensils      int    `json:"utensils"`       // utensils count: 0 for eco-friendly, -1 for on-demand, other for specified
	Invoice       bool   `json:"invoice"`        // whether invoice was issued
}

// Comment represents a comment in the comments array
type Comment struct {
	UserID    int64  `json:"userid"`    // comment user ID
	Name      string `json:"name"`      // comment user nickname
	Content   string `json:"content"`   // comment content
	Timestamp int64  `json:"timestamp"` // comment timestamp in seconds
	Choice    int    `json:"choice"`    // comment choice: 1 for support user, 2 for support merchant
	Likes     int    `json:"likes"`     // number of likes
}

// ProblemDocument represents the main Elasticsearch document model for problems
// Represents a review evaluation record
type ProblemDocument struct {
	ID          interface{} `json:"id"`           // unique identifier for the review record (string or number)
	MongoID     string      `json:"mongo_id"`     // MongoDB unique identifier
	Stars       int         `json:"stars"`        // user rating, 1-5, 1 is worst, 5 is best
	UserReview  string      `json:"user_review"`  // user's text review, may be empty for refund problems
	ReviewPics  []string    `json:"review_pics"`  // array of review image URLs
	Timestamp   int64       `json:"timestamp"`    // user review timestamp in seconds
	Others      string      `json:"others"`       // additional notes, e.g., "消费后评价"
	ProblemType int         `json:"problem_type"` // 1-delivery, 2-dine-in, 3-delivery refund, 4-dine-in refund, 5-other
	Answer      int         `json:"answer"`       // review result: 1-support user, 2-support merchant
	Ratio1      float64     `json:"ratio_1"`      // ratio of choice 1 (0~100)
	Ratio2      float64     `json:"ratio_2"`      // ratio of choice 2 (0~100)
	Uploader    string      `json:"uploader"`     // ID of user who uploaded this problem
	TaskID      string      `json:"taskId"`       // associated task ID
	UserID      string      `json:"userId"`       // original link's userID parameter
	CreatedAt   int64       `json:"created_at"`   // creation timestamp in seconds

	// Nested structures
	Replies     []Reply      `json:"replies,omitempty"`      // timeline of merchant/user replies
	Appeals     []Appeal     `json:"appeals,omitempty"`      // appeals from user/merchant
	OrderInfo   interface{}  `json:"order_info,omitempty"`   // order info for non-delivery problems (variable dict)
	Orders      []Order      `json:"orders,omitempty"`       // order items
	OrderDetail *OrderDetail `json:"order_detail,omitempty"` // order detail for delivery scenarios
	Comments    []Comment    `json:"comments,omitempty"`     // evaluation comments
}
