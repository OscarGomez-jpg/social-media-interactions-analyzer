package data

import (
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"time"

	"social-media-analyzer/pkg/models"
)

// Store holds the mock data (singleton pattern)
var (
	store *Store
	once  sync.Once
)

// Store manages the mock data
type Store struct {
	posts []models.Post
	mu    sync.RWMutex
}

// GetStore returns the singleton Store instance
func GetStore() *Store {
	once.Do(func() {
		store = &Store{
			posts: GenerateMockData(50),
		}
	})
	return store
}

// GetPosts returns all posts (thread-safe read)
func (s *Store) GetPosts() []models.Post {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := make([]models.Post, len(s.posts))
	copy(cp, s.posts)
	return cp
}

// GenerateMockData generates 50 realistic social media posts
func GenerateMockData(numRecords int) []models.Post {
	if numRecords <= 0 {
		numRecords = 50
	}

	sampleTexts := []string{
		"¿Qué piensan sobre la nueva actualización?",
		"Increíble resultado en nuestro proyecto",
		"Necesito recomendaciones de libros",
		"¿Alguien ha probado este servicio?",
		"Compartiendo mi experiencia con IA",
		"Problemas con la conectividad hoy",
		"Feliz aniversario a nuestro equipo",
		"Tutorial: cómo optimizar código en Python",
		"¿Cuál es tu herramienta favorita?",
		"Reflexión sobre el futuro del trabajo remoto",
	}

	rand.Seed(time.Now().UnixNano())
	baseTimestamp := time.Now().AddDate(0, 0, -7)
	records := make([]models.Post, 0, numRecords)
	postIDs := make([]int64, numRecords)

	// Generate post IDs
	for i := 0; i < numRecords; i++ {
		postIDs[i] = int64(i + 1)
	}

	// Create initial posts (no replies_to) - first half
	half := numRecords / 2
	for i := range half {
		postID := postIDs[i]
		timestamp := baseTimestamp.Add(time.Duration(rand.Intn(168)) * time.Hour)

		post := models.Post{
			PostID:    postID,
			UserID:    int64(rand.Intn(1001) + 1000),
			Text:      sampleTexts[rand.Intn(len(sampleTexts))],
			Likes:     rand.Intn(501),
			RepliesTo: nil,
			Timestamp: timestamp,
		}
		records = append(records, post)
	}

	// Create replies to existing posts - second half
	for i := half; i < numRecords; i++ {
		postID := postIDs[i]
		parentPostID := postIDs[rand.Intn(half)]
		timestamp := baseTimestamp.Add(time.Duration(rand.Intn(168)) * time.Hour)

		post := models.Post{
			PostID:    postID,
			UserID:    int64(rand.Intn(1001) + 1000),
			Text:      "Respuesta interesante #" + strconv.Itoa(rand.Intn(999)+1),
			Likes:     rand.Intn(201),
			RepliesTo: &parentPostID,
			Timestamp: timestamp,
		}
		records = append(records, post)
	}

	// Sort by timestamp
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	return records
}
