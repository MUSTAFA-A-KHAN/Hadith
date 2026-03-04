package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all environment configuration
type Config struct {
	// Telegram Bot
	BotToken string

	// API Settings
	APIURL        string
	APIKey        string
	APITimeout    time.Duration

	// Rate Limiting
	RateLimitRequests int
	RateLimitWindow    time.Duration

	// Logging
	LogLevel string

	// Server (for webhooks)
	ServerHost string
	ServerPort string

	// Image Cache Channel
	ImageCacheChannelID int64
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		BotToken:            getEnv("TELEGRAM_BOT_TOKEN", ""),
		ImageCacheChannelID: int64(getEnvInt("IMAGE_CACHE_CHANNEL_ID", 0)),
		APIURL:            getEnv("API_URL", "https://api.sunnah.com/v1"),
		APIKey:            getEnv("API_KEY", ""),
		APITimeout:        getEnvDuration("API_TIMEOUT", 10*time.Second),
		RateLimitRequests: getEnvInt("RATE_LIMIT_REQUESTS", 10),
		RateLimitWindow:   getEnvDuration("RATE_LIMIT_WINDOW", 1*time.Minute),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		ServerHost:        getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:        getEnv("SERVER_PORT", "8080"),
	}
}

// Validate validates the required configuration
func (c *Config) Validate() error {
	if c.BotToken == "" {
		return ErrBotTokenRequired
	}
	return nil
}

// Custom errors
var ErrBotTokenRequired = &ConfigError{"TELEGRAM_BOT_TOKEN is required"}

type ConfigError struct {
	message string
}

func (e *ConfigError) Error() string {
	return e.message
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

