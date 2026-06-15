package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	minio "github.com/minio/minio-go/v7"

	"upload-service/config"
	"upload-service/internal/api"
	"upload-service/internal/repository"
	"upload-service/internal/repository/migrations"
	"upload-service/internal/service"
	pkgMinio "upload-service/pkg/minio"
	"upload-service/pkg/rabbitmq"
)

type Container struct {
	Logger          *slog.Logger
	Config          *config.Config
	DB              *sql.DB
	MinioClient     *minio.Client
	RabbitPublisher *rabbitmq.Publisher
	Repository      *repository.Repository
	Service         *service.Service
	Handler         *api.Handler
}

func NewContainer(logger *slog.Logger, cfg *config.Config) (*Container, error) {
	if err := migrations.Run(cfg.DB.URL); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}

	db, err := sql.Open("postgres", cfg.DB.URL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	minioClient, err := pkgMinio.NewClient(context.Background(), cfg.MinIO)
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	if err := pkgMinio.EnsureBucket(context.Background(), minioClient, cfg.MinIO.Bucket); err != nil {
		return nil, fmt.Errorf("ensure bucket: %w", err)
	}

	rabbitPublisher, err := rabbitmq.NewPublisher(cfg.Rabbit.URL, cfg.Rabbit.Exchange)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq publisher: %w", err)
	}

	repo := repository.NewRepository(db)
	service := service.NewService(repo, minioClient, cfg.MinIO.Bucket, rabbitPublisher, logger)
	handler := api.NewHandler(service, logger)

	return &Container{
		Logger:          logger,
		Config:          cfg,
		DB:              db,
		MinioClient:     minioClient,
		RabbitPublisher: rabbitPublisher,
		Repository:      repo,
		Service:         service,
		Handler:         handler,
	}, nil
}

func (c *Container) Router() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/api/datasets/health", c.Handler.HealthHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/datasets/upload", c.Handler.UploadHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/datasets/{datasetId}/preview", c.Handler.PreviewHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/analysis/{datasetId}/start", c.Handler.StartAnalysisHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/analysis/{jobId}/status", c.Handler.StatusHandler).Methods(http.MethodGet)

	// serve OpenAPI spec for Swagger UI
	router.HandleFunc("/api/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs/openapi.yaml")
	}).Methods(http.MethodGet)

	return c.loggingMiddleware(router)
}

func (c *Container) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		c.Logger.Info("request complete",
			"method", r.Method,
			"path", r.URL.Path,
		)
	})
}
