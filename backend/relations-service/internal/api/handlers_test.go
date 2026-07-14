package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"relations-service/internal/domain"
	"relations-service/internal/service"
)

func TestFeaturesEndpointSmoke(t *testing.T) {
	router := testRouter()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/relations/datasets/demo/returns/return_3041/features", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var payload domain.ReturnFeaturesResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Features.CustomerReturnCount == 0 {
		t.Fatal("customerReturnCount should be present and greater than zero")
	}
	if payload.Features.AgentApprovalRate == 0 {
		t.Fatal("agentApprovalRate should be present and greater than zero")
	}
	if payload.Features.CustomerAgentPairCount == 0 {
		t.Fatal("customerAgentPairCount should be present and greater than zero")
	}
	if payload.Features.ClusterSize == 0 {
		t.Fatal("clusterSize should be present and greater than zero")
	}
	if payload.Features.RefundAmountRatio == 0 {
		t.Fatal("refundAmountRatio should be present and greater than zero")
	}
	if payload.Features.StrongestRelationType == "" {
		t.Fatal("strongestRelationType should be present")
	}
	if len(payload.Features.TopRelatedReturns) == 0 {
		t.Fatal("topRelatedReturns should be present")
	}
}

func TestScoringInputsEndpointIsDatasetScoped(t *testing.T) {
	router := testRouter()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/relations/datasets/demo/scoring-inputs", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var payload domain.ScoringInputsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.DatasetID != "demo" || len(payload.Records) != 2 || len(payload.Features) != 2 {
		t.Fatalf("unexpected scoring inputs: %+v", payload)
	}
	for _, record := range payload.Records {
		if record.DatasetID != payload.DatasetID {
			t.Fatalf("record %s escaped dataset scope", record.ReturnID)
		}
	}
}

func TestRebuildEndpointSmoke(t *testing.T) {
	router := testRouter()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/relations/datasets/demo/rebuild", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusAccepted, response.Body.String())
	}

	var payload domain.DatasetRebuildResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.RelationsCount == 0 {
		t.Fatal("relationsCount should be present and greater than zero")
	}
	if payload.FeaturesCount == 0 {
		t.Fatal("featuresCount should be present and greater than zero")
	}
}

func TestGraphEndpointSmoke(t *testing.T) {
	router := testRouter()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/relations/datasets/demo/returns/return_3041/graph", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var payload domain.GraphProjectionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Nodes) == 0 || len(payload.Edges) == 0 {
		t.Fatalf("graph should include nodes and edges: %+v", payload)
	}
}

func TestRankedAgentsEndpointSmoke(t *testing.T) {
	router := testRouter()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/relations/datasets/demo/agents/ranked?limit=1", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var payload domain.RankedAgentsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Agents) != 1 {
		t.Fatalf("agents length = %d, want 1", len(payload.Agents))
	}
}

func testRouter() http.Handler {
	records := []domain.NormalizedReturnRecord{
		{
			DatasetID:       "demo",
			ReturnID:        "return_3041",
			CustomerID:      "customer_999",
			OrderID:         "order_1041",
			SupportAgentID:  "agent_999",
			ProductCategory: "electronics",
			ReturnReason:    "item_not_as_described",
			DecisionID:      "decision_3041",
			DecisionStatus:  "APPROVED",
			RefundAmount:    1019.25,
			OrderAmount:     1168.27,
			ManualOverride:  true,
		},
		{
			DatasetID:       "demo",
			ReturnID:        "return_3042",
			CustomerID:      "customer_999",
			OrderID:         "order_1042",
			SupportAgentID:  "agent_999",
			ProductCategory: "luxury",
			ReturnReason:    "item_not_as_described",
			DecisionID:      "decision_3042",
			DecisionStatus:  "APPROVED",
			RefundAmount:    968.11,
			OrderAmount:     1182.59,
			ManualOverride:  true,
		},
	}

	handler := NewHandler(
		service.NewServiceWithRecords(noopPublisher{}, records),
		slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil)),
	)

	router := mux.NewRouter()
	router.HandleFunc("/api/relations/datasets/{datasetId}/rebuild", handler.RebuildDatasetHandler).Methods(http.MethodPost)
	router.HandleFunc("/api/relations/datasets/{datasetId}/scoring-inputs", handler.DatasetScoringInputsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/returns/{returnId}", handler.DatasetReturnRelationsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/returns/{returnId}/features", handler.DatasetReturnFeaturesHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/returns/{returnId}/related", handler.DatasetRelatedReturnsHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/returns/{returnId}/graph", handler.DatasetReturnGraphHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/customers/{customerId}/history", handler.DatasetCustomerHistoryHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/customers/{customerId}/summary", handler.DatasetCustomerSummaryHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/agents/{agentId}/summary", handler.DatasetAgentSummaryHandler).Methods(http.MethodGet)
	router.HandleFunc("/api/relations/datasets/{datasetId}/agents/ranked", handler.DatasetRankedAgentsHandler).Methods(http.MethodGet)
	return router
}

type noopPublisher struct{}

func (noopPublisher) PublishRelationsBuilt(context.Context, service.RelationsBuiltEvent) error {
	return nil
}

func (noopPublisher) PublishPipelineFailed(context.Context, service.PipelineFailedEvent) error {
	return nil
}
