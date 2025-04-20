package news

import (
	"encoding/json"
	"fmt"
	"invest-manager/internal/config"
	"net/http"
	"net/url"
	"time"
)

// Fetcher handles news API requests
type Fetcher struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// Article represents a news article
type Article struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"publishedAt"`
	Source      struct {
		Name string `json:"name"`
	} `json:"source"`
}

type newsResponse struct {
	Status       string    `json:"status"`
	TotalResults int       `json:"totalResults"`
	Articles     []Article `json:"articles"`
}

// NewFetcher creates a new news fetcher
func NewFetcher(cfg *config.Config) *Fetcher {
	return &Fetcher{
		apiKey:  cfg.NewsAPIToken,
		baseURL: "https://newsapi.org/v2/everything",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchNews fetches top news articles about Russian stocks
func (f *Fetcher) FetchNews(query string, limit int) ([]Article, error) {
	if query == "" {
		query = "Russia stocks" // Default query
	}

	if limit <= 0 {
		limit = 5 // Default limit
	}

	// Build the request URL
	reqURL, err := url.Parse(f.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	q := reqURL.Query()
	q.Set("q", query)
	q.Set("language", "en")
	q.Set("sortBy", "publishedAt")
	q.Set("pageSize", fmt.Sprintf("%d", limit))
	reqURL.RawQuery = q.Encode()

	// Create the request
	req, err := http.NewRequest(http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Add API key header
	req.Header.Add("X-Api-Key", f.apiKey)

	// Make the request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching news: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("news API returned non-OK status: %d", resp.StatusCode)
	}

	// Parse the response
	var newsResp newsResponse
	if err := json.NewDecoder(resp.Body).Decode(&newsResp); err != nil {
		return nil, fmt.Errorf("error parsing news response: %w", err)
	}

	// Check for API error status
	if newsResp.Status != "ok" {
		return nil, fmt.Errorf("news API returned error status: %s", newsResp.Status)
	}

	return newsResp.Articles, nil
} 