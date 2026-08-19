// Command server runs the Face Value HTTP API.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/isAdamBailey/face-value/backend/internal/appraisal"
	"github.com/isAdamBailey/face-value/backend/internal/auth"
	"github.com/isAdamBailey/face-value/backend/internal/config"
	"github.com/isAdamBailey/face-value/backend/internal/db"
	"github.com/isAdamBailey/face-value/backend/internal/ebay"
	"github.com/isAdamBailey/face-value/backend/internal/email"
	"github.com/isAdamBailey/face-value/backend/internal/httpapi"
	"github.com/isAdamBailey/face-value/backend/internal/pricing"
	"github.com/isAdamBailey/face-value/backend/internal/serpapi"
	"github.com/isAdamBailey/face-value/backend/internal/storage"
	"github.com/isAdamBailey/face-value/backend/internal/users"
	"github.com/isAdamBailey/face-value/backend/internal/vision"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	userRepo := users.NewPostgresRepository(queries)

	if err := userRepo.SyncAllowlist(ctx, cfg.AllowedEmails); err != nil {
		log.Fatalf("sync allowlist: %v", err)
	}

	sender, err := email.New(cfg.Email)
	if err != nil {
		log.Fatalf("email: %v", err)
	}

	authSvc := auth.NewService(queries, userRepo, sender, cfg.CookieSigningSecret, cfg.CookieSecure, cfg.AppBaseURL)

	presignTTL, err := time.ParseDuration(cfg.S3PresignTTL)
	if err != nil {
		log.Fatalf("parse S3_PRESIGN_TTL: %v", err)
	}

	imageStore, err := storage.NewS3Store(ctx, storage.S3Config{
		Bucket:          cfg.S3Bucket,
		Region:          cfg.S3Region,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
		Endpoint:        cfg.S3Endpoint,
		PublicEndpoint:  cfg.S3PublicEndpoint,
		ForcePathStyle:  cfg.S3ForcePathStyle,
		PresignTTL:      presignTTL,
	})
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	visionClient := vision.NewClient(cfg.HFAPIBase, cfg.HFToken, cfg.HFVisionModel)

	var pricingSource pricing.Source
	switch cfg.PriceSource {
	case "serpapi_ebay":
		pricingSource = serpapi.NewSource(serpapi.NewClient(cfg.SerpAPIBase, cfg.SerpAPIKey))
	default:
		pricingSource = ebay.NewSource(ebay.NewClient(cfg.EBayAPIBase, cfg.EBayClientID, cfg.EBayClientSecret))
	}

	appraisalSvc := appraisal.NewService(pool, queries, visionClient, pricingSource, appraisal.Config{
		Marketplace:   cfg.EBayMarketplace,
		CompLimit:     cfg.EBayCompLimit,
		MaxConcurrent: cfg.MaxConcurrentAppraisals,
	})

	if err := appraisalSvc.MarkStaleFailed(ctx); err != nil {
		log.Printf("mark stale searches failed: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.AppBaseURL},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	httpapi.NewHandler(authSvc, userRepo, queries, imageStore, appraisalSvc, cfg.MaxUploadBytes).Register(r)

	log.Printf("listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
