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
	RabbitConsumer  *rabbitmq.Consumer
	Repository      *repository.Repository
	Service         *service.Service
	Handler         *api.Handler
}

func NewContainer(logger *slog.Logger, cfg *config.Config) (*Container, error) {
	if err := retryDependency(logger, "postgres migrations", func() error {
		return migrations.Run(cfg.DB.URL)
	}); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}

	db, err := sql.Open("postgres", cfg.DB.URL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := retryDependency(logger, "postgres ping", func() error {
		return db.PingContext(context.Background())
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	minioClient, err := pkgMinio.NewClient(context.Background(), cfg.MinIO)
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	if err := retryDependency(logger, "minio bucket", func() error {
		return pkgMinio.EnsureBucket(context.Background(), minioClient, cfg.MinIO.Bucket)
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensure bucket: %w", err)
	}

	var rabbitPublisher *rabbitmq.Publisher
	if err := retryDependency(logger, "rabbitmq publisher", func() error {
		var err error
		rabbitPublisher, err = rabbitmq.NewPublisher(cfg.Rabbit.URL, cfg.Rabbit.Exchange)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("rabbitmq publisher: %w", err)
	}

	var rabbitConsumer *rabbitmq.Consumer
	routingKeys := []string{
		service.DatasetUploadedRoutingKey,
		service.DatasetNormalizedRoutingKey,
		service.RelationsBuiltRoutingKey,
		service.ScoringCompletedRoutingKey,
		service.PipelineFailedRoutingKey,
	}

	repo := repository.NewRepository(db)
	uploadService := service.NewService(repo, minioClient, cfg.MinIO.Bucket, rabbitPublisher, logger)
	uploadService.ConfigureUploadLimits(cfg.Upload.MaxFileSize, cfg.Upload.MaxRows, cfg.Upload.MaxErrors)
	uploadService.ConfigureNormalizationDir(cfg.Normalization.OutputDir)
	if err := retryDependency(logger, "rabbitmq lifecycle consumer", func() error {
		var consumerErr error
		rabbitConsumer, consumerErr = rabbitmq.NewConsumer(cfg.Rabbit, routingKeys, uploadService.HandleRabbitEvent, logger)
		return consumerErr
	}); err != nil {
		rabbitPublisher.Close()
		_ = db.Close()
		return nil, fmt.Errorf("rabbitmq lifecycle consumer: %w", err)
	}
	handler := api.NewHandler(uploadService, logger)

	return &Container{
		Logger:          logger,
		Config:          cfg,
		DB:              db,
		MinioClient:     minioClient,
		RabbitPublisher: rabbitPublisher,
		RabbitConsumer:  rabbitConsumer,
		Repository:      repo,
		Service:         uploadService,
		Handler:         handler,
	}, nil
}

func retryDependency(logger *slog.Logger, name string, operation func() error) error {
	const attempts = 30
	const delay = 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := operation(); err != nil {
			lastErr = err
			if attempt == attempts {
				break
			}
			logger.Warn("dependency not ready",
				"name", name,
				"attempt", attempt,
				"maxAttempts", attempts,
				"error", err,
			)
			time.Sleep(delay)
			continue
		}

		if attempt > 1 {
			logger.Info("dependency ready", "name", name, "attempt", attempt)
		}
		return nil
	}

	return lastErr
}

func (c *Container) Router() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/api/datasets/health", c.Handler.HealthHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/datasets/upload", c.Handler.UploadHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/datasets", c.Handler.ListDatasetsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/datasets/{datasetId:[0-9a-fA-F-]{36}}/preview", c.Handler.PreviewHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/datasets/{datasetId:[0-9a-fA-F-]{36}}", c.Handler.DatasetDetailsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/datasets/{datasetId:[0-9a-fA-F-]{36}}/archive", c.Handler.ArchiveDatasetHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/analysis/{datasetId:[0-9a-fA-F-]{36}}/start", c.Handler.StartAnalysisHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/analysis/{jobId:[0-9a-fA-F-]{36}}/status", c.Handler.StatusHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/analysis/{jobId:[0-9a-fA-F-]{36}}/retry", c.Handler.RetryAnalysisHandler).Methods(http.MethodPost)
	if c.Config.Admin.StatusPatchEnabled {
		router.HandleFunc("/api/analysis/{jobId:[0-9a-fA-F-]{36}}/status", c.Handler.UpdateStatusHandler).Methods(http.MethodPatch)
	}

	// serve OpenAPI spec for Swagger UI
	router.HandleFunc("/api/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs/openapi.yaml")
	}).Methods(http.MethodGet)

	return c.loggingMiddleware(router)
}

func (c *Container) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)
		c.Logger.Info("request complete",
			"method", r.Method,
			"path", r.URL.Path,
			"status", writer.status,
			"durationMs", time.Since(started).Milliseconds(),
		)
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
