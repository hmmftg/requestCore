package libCallApi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

// FakeAPIServer creates a local test server that mimics external APIs
type FakeAPIServer struct {
	server *httptest.Server
}

// AnimeEpisode represents a single anime episode
type AnimeEpisode struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// AnimeEpisodesResponse represents the response structure from Jikan API
type AnimeEpisodesResponse struct {
	Data       []AnimeEpisode `json:"data"`
	Pagination struct {
		LastVisiblePage int  `json:"last_visible_page"`
		HasNextPage     bool `json:"has_next_page"`
	} `json:"pagination"`
}

// NewFakeAPIServer creates a new fake API server for testing
func NewFakeAPIServer() *FakeAPIServer {
	mux := http.NewServeMux()

	// Mock generic API endpoints
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		// Extract query from path - the path will be /api/test1, /api/test2, etc.
		path := strings.TrimPrefix(r.URL.Path, "/api/")

		// Generate mock response based on query
		response := generateMockSimpleResponse(path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Mock Jikan API endpoints (for backward compatibility)
	mux.HandleFunc("/v4/anime/", func(w http.ResponseWriter, r *http.Request) {
		// Extract anime ID from path
		path := strings.TrimPrefix(r.URL.Path, "/v4/anime/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		animeID := parts[0]
		endpoint := parts[1]

		if endpoint != "episodes" {
			http.Error(w, "Unknown endpoint", http.StatusNotFound)
			return
		}

		// Generate mock response based on anime ID
		response := generateMockAnimeEpisodes(animeID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Generic mock endpoint for other APIs
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Mock API Response",
			"path":    r.URL.Path,
			"method":  r.Method,
		})
	})

	server := httptest.NewServer(mux)
	return &FakeAPIServer{server: server}
}

// URL returns the base URL of the fake server
func (f *FakeAPIServer) URL() string {
	return f.server.URL
}

// Close shuts down the fake server
func (f *FakeAPIServer) Close() {
	f.server.Close()
}

// SimpleTestData represents a simplified test response structure
type SimpleTestData struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SimpleTestResponse represents a simplified API response
type SimpleTestResponse struct {
	Data   []SimpleTestData `json:"data"`
	Status string           `json:"status"`
	Count  int              `json:"count"`
}

// generateMockSimpleResponse generates simplified mock responses for generic API testing
func generateMockSimpleResponse(query string) SimpleTestResponse {
	switch query {
	case "test1":
		return SimpleTestResponse{
			Data: []SimpleTestData{
				{ID: 1, Name: "Test Item 1", Value: "Value 1"},
				{ID: 2, Name: "Test Item 2", Value: "Value 2"},
			},
			Status: "success",
			Count:  2,
		}
	case "test2":
		return SimpleTestResponse{
			Data: []SimpleTestData{
				{ID: 3, Name: "Test Item 3", Value: "Value 3"},
				{ID: 4, Name: "Test Item 4", Value: "Value 4"},
			},
			Status: "success",
			Count:  2,
		}
	case "test3":
		return SimpleTestResponse{
			Data: []SimpleTestData{
				{ID: 5, Name: "Test Item 5", Value: "Value 5"},
			},
			Status: "success",
			Count:  1,
		}
	default:
		return SimpleTestResponse{
			Data: []SimpleTestData{
				{ID: 0, Name: "Default Item", Value: "Default Value"},
			},
			Status: "success",
			Count:  1,
		}
	}
}

// generateMockAnimeEpisodes generates simplified mock anime episodes based on anime ID
func generateMockAnimeEpisodes(animeID string) AnimeEpisodesResponse {
	id, _ := strconv.Atoi(animeID)

	var episodes []AnimeEpisode

	switch id {
	case 1: // Cowboy Bebop - Simplified test data
		episodes = []AnimeEpisode{
			{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/1", Title: "Asteroid Blues"},
			{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/2", Title: "Stray Dog Strut"},
			{URL: "https://myanimelist.net/anime/1/Cowboy_Bebop/episode/3", Title: "Honky Tonk Women"},
		}
	case 200: // Angel Beats - Simplified test data
		episodes = []AnimeEpisode{
			{Title: "Meeting at Full Speed − Is the Angel Male or Female?"},
			{Title: "What's Wrong? My Angel!"},
			{Title: "This Is the Man's Hand That Defeats Enemies with a Single Blow!"},
		}
	case 300: // Death Note - Simplified test data
		episodes = []AnimeEpisode{
			{Title: "Transmigration"},
			{Title: "Yakumo"},
			{Title: "Sacrifice"},
			{Title: "Straying"},
		}
	case 400: // Outlaw Star - Simplified test data
		episodes = []AnimeEpisode{
			{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/1", Title: "Outlaw World"},
			{URL: "https://myanimelist.net/anime/400/Seihou_Bukyou_Outlaw_Star/episode/2", Title: "Star of Desire"},
		}
	default:
		// Default response for unknown anime IDs
		episodes = []AnimeEpisode{
			{Title: fmt.Sprintf("Episode 1 of Anime %s", animeID)},
			{Title: fmt.Sprintf("Episode 2 of Anime %s", animeID)},
		}
	}

	return AnimeEpisodesResponse{
		Data: episodes,
		Pagination: struct {
			LastVisiblePage int  `json:"last_visible_page"`
			HasNextPage     bool `json:"has_next_page"`
		}{
			LastVisiblePage: 1,
			HasNextPage:     false,
		},
	}
}

// MockErrorResponse creates a mock error response
func MockErrorResponse(statusCode int, message string) map[string]interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"status":  statusCode,
			"message": message,
		},
	}
}

// MockSuccessResponse creates a generic success response
func MockSuccessResponse(data interface{}) map[string]interface{} {
	return map[string]interface{}{
		"status": "success",
		"data":   data,
	}
}
