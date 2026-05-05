package models

import "time"

// Post represents a social media post
type Post struct {
	PostID    int64     `json:"post_id"`
	UserID    int64     `json:"user_id"`
	Text      string    `json:"text"`
	Likes     int       `json:"likes"`
	RepliesTo *int64    `json:"replies_to"`
	Timestamp time.Time `json:"timestamp"`
}

// TopPost represents a post in the summary
type TopPost struct {
	PostID int64  `json:"post_id"`
	Likes  int    `json:"likes"`
	Text   string `json:"text"`
	UserID int64  `json:"user_id"`
}

// SummaryResult is the output of the summary tool
type SummaryResult struct {
	TotalPostsAnalyzed int       `json:"total_posts_analyzed"`
	SummaryPostsCount  int       `json:"summary_posts_count"`
	AverageLikes       float64   `json:"average_likes"`
	AvgRepliesPerPost  float64   `json:"average_replies_per_post"`
	TopPosts           []TopPost `json:"top_posts"`
	KeyInsight         string    `json:"key_insight"`
}

// MetricsResult is the output of the metrics tool
type MetricsResult struct {
	MetricType          string   `json:"metric_type"`
	MostLikedPost       *TopPost `json:"most_liked_post,omitempty"`
	TotalLikesInDataset int      `json:"total_likes_in_dataset,omitempty"`
	MedianLikes         int      `json:"median_likes,omitempty"`
	MostActiveUserID    int64    `json:"most_active_user_id,omitempty"`
	MostActiveUserPosts int      `json:"most_active_user_posts,omitempty"`
	TotalUniqueUsers    int      `json:"total_unique_users,omitempty"`
	AveragePostsPerUser float64  `json:"average_posts_per_user,omitempty"`
}

// PropagationResult is the output of the propagation analysis tool
type PropagationResult struct {
	PostID                    int64   `json:"post_id"`
	OriginalPostUser          int64   `json:"original_post_user"`
	OriginalPostText          string  `json:"original_post_text"`
	DirectReplies             int     `json:"direct_replies"`
	IndirectReplies           int     `json:"indirect_replies"`
	TotalReach                int     `json:"total_reach"`
	ReachDepth                int     `json:"reach_depth"`
	AverageResponseSpeedHours float64 `json:"average_response_speed_hours"`
	EngagementRate            float64 `json:"engagement_rate"`
	DirectRepliesList         []int64 `json:"direct_replies_list"`
	IndirectRepliesList       []int64 `json:"indirect_replies_list"`
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ToolName string `json:"tool_name"`
	Status   string `json:"status"` // "success" or "error"
	Result   any    `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
}
