package config

import (
	"errors"
	"os"
	"time"
)

// Config stores all configuration for the application
type Config struct {
	TinkoffToken    string
	TinkoffAccountID string
	TinkoffEndpoint string
	OpenAIApiKey    string
	TelegramToken   string
	TelegramChatID  string
	NewsAPIToken    string
	Timezone        *time.Location
	LogLevel        string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		TinkoffToken:    os.Getenv("TINKOFF_TOKEN"),
		TinkoffAccountID: os.Getenv("TINKOFF_ACCOUNT_ID"),
		TinkoffEndpoint: os.Getenv("TINKOFF_ENDPOINT"),
		OpenAIApiKey:    os.Getenv("OPENAI_API_KEY"),
		TelegramToken:   os.Getenv("TELEGRAM_TOKEN"),
		TelegramChatID:  os.Getenv("TELEGRAM_CHAT_ID"),
		NewsAPIToken:    os.Getenv("NEWSAPI_TOKEN"),
		LogLevel:        getEnvOrDefault("LOG_LEVEL", "info"),
	}

	// Load timezone
	tzName := os.Getenv("TIMEZONE")
	if tzName == "" {
		tzName = "Europe/Moscow" // Default to Moscow time
	}

	location, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, err
	}
	cfg.Timezone = location

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// validate checks if all required fields are provided
func (c *Config) validate() error {
	if c.TinkoffToken == "" {
		return errors.New("TINKOFF_TOKEN is required")
	}
	if c.TinkoffAccountID == "" {
		return errors.New("TINKOFF_ACCOUNT_ID is required")
	}
	if c.OpenAIApiKey == "" {
		return errors.New("OPENAI_API_KEY is required")
	}
	if c.TelegramToken == "" {
		return errors.New("TELEGRAM_TOKEN is required")
	}
	if c.TelegramChatID == "" {
		return errors.New("TELEGRAM_CHAT_ID is required")
	}
	if c.NewsAPIToken == "" {
		return errors.New("NEWSAPI_TOKEN is required")
	}
	return nil
} 