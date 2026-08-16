// Package config loads and validates application configuration from
// environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the server.
type Config struct {
	Port                string
	DatabaseURL         string
	AppBaseURL          string
	CookieSigningSecret []byte
	CookieSecure        bool

	HFToken      string
	HFVisionModel string
	HFAPIBase    string

	EBayClientID     string
	EBayClientSecret string
	EBayAPIBase      string
	EBayMarketplace  string
	EBayCompLimit    int
	PriceSource      string

	S3Bucket          string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3Endpoint        string
	S3ForcePathStyle  bool
	S3PresignTTL      string

	MaxUploadBytes        int64
	MaxConcurrentAppraisals int
}

// Load reads configuration from environment variables, applying defaults
// where appropriate and returning an error if any required variable is
// missing. A daemon that starts and then fails every request is worse than
// one that won't start, so every dependency the pipeline needs is validated
// here rather than lazily at first use.
func Load() (Config, error) {
	var missing []string
	require := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			missing = append(missing, name)
		}
		return v
	}

	cfg := Config{
		Port:                envOrDefault("PORT", "8080"),
		DatabaseURL:         require("DATABASE_URL"),
		AppBaseURL:          envOrDefault("APP_BASE_URL", "http://localhost:3000"),
		CookieSigningSecret: []byte(require("COOKIE_SIGNING_SECRET")),
		CookieSecure:        os.Getenv("COOKIE_SECURE") == "true",

		HFToken:       require("HF_TOKEN"),
		HFVisionModel: envOrDefault("HF_VISION_MODEL", "Qwen/Qwen2.5-VL-72B-Instruct"),
		HFAPIBase:     envOrDefault("HF_API_BASE", "https://router.huggingface.co/v1"),

		EBayClientID:     require("EBAY_CLIENT_ID"),
		EBayClientSecret: require("EBAY_CLIENT_SECRET"),
		EBayAPIBase:      envOrDefault("EBAY_API_BASE", "https://api.sandbox.ebay.com"),
		EBayMarketplace:  envOrDefault("EBAY_MARKETPLACE_ID", "EBAY_US"),
		PriceSource:      envOrDefault("PRICE_SOURCE", "ebay_browse"),

		S3Bucket:          require("S3_BUCKET"),
		S3Region:          require("S3_REGION"),
		S3AccessKeyID:     require("S3_ACCESS_KEY_ID"),
		S3SecretAccessKey: require("S3_SECRET_ACCESS_KEY"),
		S3Endpoint:        os.Getenv("S3_ENDPOINT"),
		S3ForcePathStyle:  os.Getenv("S3_FORCE_PATH_STYLE") == "true",
		S3PresignTTL:      envOrDefault("S3_PRESIGN_TTL", "15m"),
	}

	cfg.EBayCompLimit = envOrDefaultInt("EBAY_COMP_LIMIT", 50)
	cfg.MaxUploadBytes = envOrDefaultInt64("MAX_UPLOAD_BYTES", 10*1024*1024)
	cfg.MaxConcurrentAppraisals = envOrDefaultInt("MAX_CONCURRENT_APPRAISALS", 4)

	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func envOrDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func envOrDefaultInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envOrDefaultInt64(name string, def int64) int64 {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
