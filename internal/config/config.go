// Package config reads the configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"strconv"
)

type Config struct {
	Addr          string
	DatabaseURL   string
	JWTSecret     string
	AdminEmail    string
	AdminPassword string
	DemoEnabled   bool
	Explainer     string // "openai" or "fake" (tests, CI, local development)
	OpenAIKey     string
	OpenAIBaseURL string
	OpenAIModel   string
	DailyLimit    int
}

// Load takes getenv as a parameter so tests can pass a map instead of touching the process environment.
func Load(getenv func(string) string) (Config, error) {
	or := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}
	c := Config{
		Addr:          or("ADDR", ":8080"),
		DatabaseURL:   getenv("DATABASE_URL"),
		JWTSecret:     getenv("JWT_SECRET"),
		AdminEmail:    getenv("ADMIN_EMAIL"),
		AdminPassword: getenv("ADMIN_PASSWORD"),
		DemoEnabled:   getenv("DEMO_ENABLED") == "true",
		Explainer:     or("EXPLAINER", "openai"),
		OpenAIKey:     getenv("OPENAI_API_KEY"),
		OpenAIBaseURL: or("AI_BASE_URL", "https://api.openai.com"),
		OpenAIModel:   or("AI_MODEL", "gpt-4o-mini"),
	}
	limit, err := strconv.Atoi(or("DAILY_LIMIT", "20"))
	if err != nil || limit < 0 {
		return Config{}, fmt.Errorf("DAILY_LIMIT must be a non-negative number")
	}
	c.DailyLimit = limit
	switch {
	case c.DatabaseURL == "":
		return Config{}, errors.New("DATABASE_URL is required")
	case len(c.JWTSecret) < 32:
		return Config{}, errors.New("JWT_SECRET must be at least 32 bytes")
	case c.Explainer != "openai" && c.Explainer != "fake":
		return Config{}, errors.New(`EXPLAINER must be "openai" or "fake"`)
	}
	return c, nil
}
