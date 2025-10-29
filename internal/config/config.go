package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config stores runtime configuration loaded from environment variables.
type Config struct {
	Port                 string
	TwilioAccountSID     string
	TwilioAuthToken      string
	TwilioWhatsAppNumber string
	OpenAIAPIKey         string
	DatabaseURL          string
	LocalTimezone        *time.Location
}

// Load reads configuration values and prepares defaults where applicable.
func Load() *Config {
	_ = godotenv.Load()

	port := getenvDefault("PORT", "8080")
	accountSID := os.Getenv("TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	whatsAppNumber := os.Getenv("TWILIO_WHATSAPP_NUMBER")
	openAIKey := os.Getenv("OPENAI_API_KEY")
	databaseURL := os.Getenv("DATABASE_URL")
	timezoneName := getenvDefault("LOCAL_TIMEZONE", "Local")

	location, err := time.LoadLocation(timezoneName)
	if err != nil {
		log.Printf("config: invalid LOCAL_TIMEZONE %q, defaulting to system local: %v", timezoneName, err)
		location = time.Local
	}

	return &Config{
		Port:                 port,
		TwilioAccountSID:     accountSID,
		TwilioAuthToken:      authToken,
		TwilioWhatsAppNumber: whatsAppNumber,
		OpenAIAPIKey:         openAIKey,
		DatabaseURL:          databaseURL,
		LocalTimezone:        location,
	}
}

func getenvDefault(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	return value
}

// ParseIntEnv returns the integer value for an environment variable or the provided default.
func ParseIntEnv(key string, def int) int {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("config: unable to parse %s=%q as int: %v", key, value, err)
		return def
	}
	return parsed
}
