package app

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"relations-service/config"
	"relations-service/internal/api"
	"relations-service/internal/service"
	"relations-service/pkg/rabbitmq"
)

type Container struct {
	Logger  *slog.Logger
	Config  *config.Config
	Rabbit  *rabbitmq.Client
	Service *service.Service
	Handler *api.Handler
}

func NewContainer(logger *slog.Logger, cfg *config.Config) (*Container, error) {
	var rabbitClient *rabbitmq.Client
	var publisher service.RelationsBuiltPublisher

	if cfg.Rabbit.Enabled {
		client, err := rabbitmq.NewClient(cfg.Rabbit)
		if err != nil {
			return nil, err
		}
		rabbitClient = client
		publisher = client
	}

	relationsService := service.NewService(publisher, service.Options{
		DatasetID:         cfg.Data.DatasetID,
		DatasetPath:       cfg.Data.DatasetPath,
		AllowDemoFallback: cfg.Data.DemoFallbackEnabled,
	})
	handler := api.NewHandler(relationsService, logger)

	return &Container{
		Logger:  logger,
		Config:  cfg,
		Rabbit:  rabbitClient,
		Service: relationsService,
		Handler: handler,
	}, nil
}

func (c *Container) Router() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/api/relations/health", c.Handler.HealthHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/rebuild", c.Handler.RebuildDatasetHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/relations/datasets/{datasetId}/scoring-inputs", c.Handler.DatasetScoringInputsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/returns/{returnId}", c.Handler.DatasetReturnRelationsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/returns/{returnId}/features", c.Handler.DatasetReturnFeaturesHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/returns/{returnId}/related", c.Handler.DatasetRelatedReturnsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/returns/{returnId}/graph", c.Handler.DatasetReturnGraphHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/customers/{customerId}/history", c.Handler.DatasetCustomerHistoryHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/customers/{customerId}/summary", c.Handler.DatasetCustomerSummaryHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/agents/{agentId}/summary", c.Handler.DatasetAgentSummaryHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/agents/ranked", c.Handler.DatasetRankedAgentsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/returns/{returnId}", c.Handler.ReturnRelationsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/returns/{returnId}/features", c.Handler.ReturnFeaturesHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/customers/{customerId}/history", c.Handler.CustomerHistoryHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/agents/{agentId}/summary", c.Handler.AgentSummaryHandler).Methods(http.MethodGet)

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
