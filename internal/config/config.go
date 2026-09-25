package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// AppConfig holds all runtime settings for the file host and OnionGuard integration.
type AppConfig struct {
	ListenAddr      string
	DataDir         string
	UploadDir       string
	MetaFilePath    string
	MaxUploadBytes  int64
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
	APIToken        string
	WaitRoomEnabled bool
	WaitRoomSeconds time.Duration
	CaptchaEnabled  bool
	SiteTitle       string
}

// LoadConfig reads configuration parameters from environment variables with sensible defaults.
func LoadConfig() *AppConfig {
	addr := getEnv("LISTEN_ADDR", ":8080")
	dataDir := getEnv("DATA_DIR", "./data")
	uploadDir := filepath.Join(dataDir, "uploads")
	metaFilePath := filepath.Join(dataDir, "meta.json")

	maxUploadMB := getEnvInt("MAX_UPLOAD_MB", 25)
	maxUploadBytes := int64(maxUploadMB) * 1024 * 1024

	waitSec := getEnvInt("WAIT_ROOM_SECONDS", 3)
	waitRoomEnabled := getEnvBool("WAIT_ROOM_ENABLED", true)
	captchaEnabled := getEnvBool("CAPTCHA_ENABLED", true)

	return &AppConfig{
		ListenAddr:      addr,
		DataDir:         dataDir,
		UploadDir:       uploadDir,
		MetaFilePath:    metaFilePath,
		MaxUploadBytes:  maxUploadBytes,
		DefaultTTL:      24 * time.Hour,
		CleanupInterval: 5 * time.Minute,
		APIToken:        os.Getenv("API_BEARER_TOKEN"),
		WaitRoomEnabled: waitRoomEnabled,
		WaitRoomSeconds: time.Duration(waitSec) * time.Second,
		CaptchaEnabled:  captchaEnabled,
		SiteTitle:       getEnv("SITE_TITLE", "OnionGuard File Host (Zero-JS)"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}
