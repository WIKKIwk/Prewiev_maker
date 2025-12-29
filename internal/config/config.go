package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramToken string
	GeminiAPIKey  string

	LogLevel string
	Debug    bool

	PreferIPv4 bool

	MediaGroupDebounce time.Duration
	MaxConcurrent      int
	MaxHistoryMessages int
	RequestTimeout     time.Duration
	HTTPTimeout        time.Duration
	GeminiBaseURL      string
	GeminiAPIVersion   string
}

func Load() (Config, error) {
	cfg := Config{
		LogLevel:           strings.ToLower(strings.TrimSpace(getEnv("LOG_LEVEL", "info"))),
		Debug:              getEnvBool("DEBUG", false),
		PreferIPv4:         getEnvBool("PREFER_IPV4", true),
		MediaGroupDebounce: time.Duration(getEnvInt("MEDIA_GROUP_DEBOUNCE_MS", 1200)) * time.Millisecond,
		MaxConcurrent:      getEnvInt("MAX_CONCURRENT", 4),
		MaxHistoryMessages: getEnvInt("MAX_HISTORY_MESSAGES", 20),
		RequestTimeout:     time.Duration(getEnvInt("REQUEST_TIMEOUT_SECONDS", 180)) * time.Second,
		HTTPTimeout:        time.Duration(getEnvInt("HTTP_TIMEOUT_SECONDS", 180)) * time.Second,
		GeminiBaseURL:      strings.TrimSpace(getEnv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com")),
		GeminiAPIVersion:   strings.TrimSpace(getEnv("GEMINI_API_VERSION", "v1beta")),
	}

	cfg.TelegramToken = strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	cfg.GeminiAPIKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))

	switch {
	case cfg.TelegramToken == "":
		return Config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	case cfg.GeminiAPIKey == "":
		return Config{}, errors.New("GEMINI_API_KEY is required")
	}

	if cfg.MaxConcurrent < 1 {
		cfg.MaxConcurrent = 1
	}
	if cfg.MaxHistoryMessages < 1 {
		cfg.MaxHistoryMessages = 1
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 180 * time.Second
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 180 * time.Second
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
