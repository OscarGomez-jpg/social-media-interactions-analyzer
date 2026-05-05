package tools

import (
	"fmt"
	"sort"

	"social-media-analyzer/pkg/data"
	"social-media-analyzer/pkg/models"
)

// Tools executes the 3 social listening tools
type Tools struct {
	store *data.Store
}

// NewTools creates a new Tools instance
func NewTools() *Tools {
	return &Tools{
		store: data.GetStore(),
	}
}

// GetConversationSummary returns a summary of top posts
func (t *Tools) GetConversationSummary(numPosts int) *models.SummaryResult {
	if numPosts < 1 || numPosts > 50 {
		numPosts = 10
	}

	posts := t.store.GetPosts()

	// Sort by likes descending
	sortedPosts := make([]models.Post, len(posts))
	copy(sortedPosts, posts)

	sort.Slice(sortedPosts, func(i, j int) bool {
		return sortedPosts[i].Likes > sortedPosts[j].Likes
	})

	topN := numPosts
	if topN > len(sortedPosts) {
		topN = len(sortedPosts)
	}

	topPosts := sortedPosts[:topN]

	// Calculate metrics
	totalLikes := 0
	for _, p := range posts {
		totalLikes += p.Likes
	}

	// Count replies per post
	repliesMap := make(map[int64]int)
	for _, p := range posts {
		if p.RepliesTo != nil {
			repliesMap[*p.RepliesTo]++
		}
	}

	avgRepliesPerPost := 0.0
	if len(repliesMap) > 0 {
		totalReplies := 0
		for _, count := range repliesMap {
			totalReplies += count
		}
		avgRepliesPerPost = float64(totalReplies) / float64(len(repliesMap))
	}

	// Build top posts response
	topPostsResponse := make([]models.TopPost, len(topPosts))
	topLikesSum := 0
	for i, p := range topPosts {
		topPostsResponse[i] = models.TopPost{
			PostID: p.PostID,
			Likes:  p.Likes,
			Text:   p.Text,
			UserID: p.UserID,
		}
		topLikesSum += p.Likes
	}

	avgTopLikes := 0.0
	if len(topPosts) > 0 {
		avgTopLikes = float64(topLikesSum) / float64(len(topPosts))
	}

	return &models.SummaryResult{
		TotalPostsAnalyzed: len(posts),
		SummaryPostsCount:  topN,
		AverageLikes:       float64(totalLikes) / float64(len(posts)),
		AvgRepliesPerPost:  avgRepliesPerPost,
		TopPosts:           topPostsResponse,
		KeyInsight:         fmt.Sprintf("Los posts con mayor engagement tienen en promedio %.0f likes", avgTopLikes),
	}
}

// GetSocialMetrics returns engagement or user activity metrics
func (t *Tools) GetSocialMetrics(metricType string) *models.MetricsResult {
	posts := t.store.GetPosts()

	switch metricType {
	case "top_engagement":
		return t.topEngagementMetrics(posts)
	case "user_activity":
		return t.userActivityMetrics(posts)
	}

	return &models.MetricsResult{
		MetricType: "error",
	}
}

func (t *Tools) topEngagementMetrics(posts []models.Post) *models.MetricsResult {
	if len(posts) == 0 {
		return &models.MetricsResult{MetricType: "top_engagement"}
	}

	// Find most liked post
	maxIdx := 0
	for i := 1; i < len(posts); i++ {
		if posts[i].Likes > posts[maxIdx].Likes {
			maxIdx = i
		}
	}

	mostLiked := posts[maxIdx]
	totalLikes := 0
	likes := make([]int, len(posts))

	for i, p := range posts {
		totalLikes += p.Likes
		likes[i] = p.Likes
	}

	// Calculate median
	sort.Ints(likes)
	median := likes[len(likes)/2]

	return &models.MetricsResult{
		MetricType: "top_engagement",
		MostLikedPost: &models.TopPost{
			PostID: mostLiked.PostID,
			Likes:  mostLiked.Likes,
			UserID: mostLiked.UserID,
			Text:   mostLiked.Text,
		},
		TotalLikesInDataset: totalLikes,
		MedianLikes:         median,
	}
}

func (t *Tools) userActivityMetrics(posts []models.Post) *models.MetricsResult {
	if len(posts) == 0 {
		return &models.MetricsResult{MetricType: "user_activity"}
	}

	// Count posts per user
	userActivity := make(map[int64]int)
	for _, p := range posts {
		userActivity[p.UserID]++
	}

	// Find most active user
	var mostActiveUserID int64
	maxCount := 0
	for userID, count := range userActivity {
		if count > maxCount {
			maxCount = count
			mostActiveUserID = userID
		}
	}

	// Calculate average
	totalPosts := len(posts)
	avgPostsPerUser := float64(totalPosts) / float64(len(userActivity))

	return &models.MetricsResult{
		MetricType:          "user_activity",
		MostActiveUserID:    mostActiveUserID,
		MostActiveUserPosts: maxCount,
		TotalUniqueUsers:    len(userActivity),
		AveragePostsPerUser: avgPostsPerUser,
	}
}

// AnalyzePropagation analyzes how a post propagates
func (t *Tools) AnalyzePropagation(postID int64) *models.PropagationResult {
	posts := t.store.GetPosts()

	// Find original post
	var originalPost *models.Post
	for i := range posts {
		if posts[i].PostID == postID {
			originalPost = &posts[i]
			break
		}
	}

	if originalPost == nil {
		return &models.PropagationResult{
			PostID: postID,
		}
	}

	// Find direct replies
	directReplies := make([]models.Post, 0)
	directReplyIDs := make([]int64, 0)

	for _, p := range posts {
		if p.RepliesTo != nil && *p.RepliesTo == postID {
			directReplies = append(directReplies, p)
			directReplyIDs = append(directReplyIDs, p.PostID)
		}
	}

	// Find indirect replies
	indirectReplies := make([]models.Post, 0)
	indirectReplyIDs := make([]int64, 0)

	for _, p := range posts {
		if p.RepliesTo != nil {
			for _, directID := range directReplyIDs {
				if *p.RepliesTo == directID {
					indirectReplies = append(indirectReplies, p)
					indirectReplyIDs = append(indirectReplyIDs, p.PostID)
					break
				}
			}
		}
	}

	// Calculate average response speed
	avgResponseSpeedHours := 0.0
	if len(directReplies) > 0 {
		totalDuration := 0.0
		for _, reply := range directReplies {
			duration := reply.Timestamp.Sub(originalPost.Timestamp)
			hours := duration.Hours()
			totalDuration += hours
		}
		avgResponseSpeedHours = totalDuration / float64(len(directReplies))
	}

	// Calculate reach depth
	reachDepth := 0
	if len(directReplies) > 0 {
		reachDepth = 1
		if len(indirectReplies) > 0 {
			reachDepth = 2
		}
	}

	// Calculate engagement rate
	engagementRate := 0.0
	if len(posts) > 0 {
		engagementRate = float64(len(directReplies)) / float64(len(posts)) * 100
	}

	// Truncate text
	originalText := originalPost.Text
	if len(originalText) > 100 {
		originalText = originalText[:100]
	}

	return &models.PropagationResult{
		PostID:                    postID,
		OriginalPostUser:          originalPost.UserID,
		OriginalPostText:          originalText,
		DirectReplies:             len(directReplies),
		IndirectReplies:           len(indirectReplies),
		TotalReach:                len(directReplies) + len(indirectReplies),
		ReachDepth:                reachDepth,
		AverageResponseSpeedHours: float64(int(avgResponseSpeedHours*100)) / 100,
		EngagementRate:            float64(int(engagementRate*100)) / 100,
		DirectRepliesList:         directReplyIDs,
		IndirectRepliesList:       indirectReplyIDs,
	}
}
