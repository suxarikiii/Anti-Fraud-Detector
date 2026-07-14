package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"relations-service/internal/service"

	"github.com/gorilla/mux"
)

type Handler struct {
	Service *service.Service
	Logger  *slog.Logger
}

type response struct {
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func NewHandler(service *service.Service, logger *slog.Logger) *Handler {
	return &Handler{Service: service, Logger: logger}
}

func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Service.Health())
}

func (h *Handler) RebuildDatasetHandler(w http.ResponseWriter, r *http.Request) {
	datasetID := mux.Vars(r)["datasetId"]
	if datasetID == "" {
		writeError(w, http.StatusBadRequest, "dataset id is required")
		return
	}

	rebuild, err := h.Service.RebuildDataset(r.Context(), datasetID)
	if err != nil {
		if errors.Is(err, service.ErrDatasetNotFound) {
			writeError(w, http.StatusNotFound, "dataset %s not found", datasetID)
			return
		}
		h.Logger.Error("rebuild relations error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to rebuild relations: %v", err)
		return
	}

	writeJSON(w, http.StatusAccepted, rebuild)
}

func (h *Handler) ReturnRelationsHandler(w http.ResponseWriter, r *http.Request) {
	returnID := mux.Vars(r)["returnId"]
	if returnID == "" {
		writeError(w, http.StatusBadRequest, "return id is required")
		return
	}

	relations, err := h.Service.GetReturnRelations(returnID)
	if err != nil {
		if errors.Is(err, service.ErrReturnNotFound) {
			writeError(w, http.StatusNotFound, "return %s not found", returnID)
			return
		}
		h.Logger.Error("return relations error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get return relations: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, relations)
}

func (h *Handler) DatasetReturnRelationsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID := vars["datasetId"]
	returnID := vars["returnId"]
	if datasetID == "" || returnID == "" {
		writeError(w, http.StatusBadRequest, "dataset id and return id are required")
		return
	}

	relations, err := h.Service.GetReturnRelationsForDataset(datasetID, returnID)
	if err != nil {
		writeLookupError(w, err, "dataset %s or return %s not found", datasetID, returnID)
		return
	}

	writeJSON(w, http.StatusOK, relations)
}

func (h *Handler) CustomerHistoryHandler(w http.ResponseWriter, r *http.Request) {
	customerID := mux.Vars(r)["customerId"]
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "customer id is required")
		return
	}

	writeJSON(w, http.StatusOK, h.Service.GetCustomerHistory(customerID))
}

func (h *Handler) DatasetCustomerHistoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID := vars["datasetId"]
	customerID := vars["customerId"]
	if datasetID == "" || customerID == "" {
		writeError(w, http.StatusBadRequest, "dataset id and customer id are required")
		return
	}

	history, err := h.Service.GetCustomerHistoryForDataset(datasetID, customerID)
	if err != nil {
		writeLookupError(w, err, "dataset %s not found", datasetID)
		return
	}

	writeJSON(w, http.StatusOK, history)
}

func (h *Handler) DatasetCustomerSummaryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID := vars["datasetId"]
	customerID := vars["customerId"]
	if datasetID == "" || customerID == "" {
		writeError(w, http.StatusBadRequest, "dataset id and customer id are required")
		return
	}

	summary, err := h.Service.GetCustomerBehaviorSummary(datasetID, customerID, queryInt(r, "limit", 10))
	if err != nil {
		writeLookupError(w, err, "dataset %s not found", datasetID)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) AgentSummaryHandler(w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agentId"]
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	writeJSON(w, http.StatusOK, h.Service.GetAgentSummary(agentID))
}

func (h *Handler) DatasetAgentSummaryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID := vars["datasetId"]
	agentID := vars["agentId"]
	if datasetID == "" || agentID == "" {
		writeError(w, http.StatusBadRequest, "dataset id and agent id are required")
		return
	}

	summary, err := h.Service.GetAgentSummaryForDataset(datasetID, agentID)
	if err != nil {
		writeLookupError(w, err, "dataset %s not found", datasetID)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) DatasetRankedAgentsHandler(w http.ResponseWriter, r *http.Request) {
	datasetID := mux.Vars(r)["datasetId"]
	if datasetID == "" {
		writeError(w, http.StatusBadRequest, "dataset id is required")
		return
	}

	agents, err := h.Service.GetRankedAgents(datasetID, queryInt(r, "limit", 10), r.URL.Query().Get("sort"))
	if err != nil {
		writeLookupError(w, err, "dataset %s not found", datasetID)
		return
	}

	writeJSON(w, http.StatusOK, agents)
}

func (h *Handler) ReturnFeaturesHandler(w http.ResponseWriter, r *http.Request) {
	returnID := mux.Vars(r)["returnId"]
	if returnID == "" {
		writeError(w, http.StatusBadRequest, "return id is required")
		return
	}

	features, err := h.Service.GetReturnFeatures(returnID)
	if err != nil {
		if errors.Is(err, service.ErrReturnNotFound) {
			writeError(w, http.StatusNotFound, "return %s not found", returnID)
			return
		}
		h.Logger.Error("return features error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get return features: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, features)
}

func (h *Handler) DatasetRelatedReturnsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID := vars["datasetId"]
	returnID := vars["returnId"]
	if datasetID == "" || returnID == "" {
		writeError(w, http.StatusBadRequest, "dataset id and return id are required")
		return
	}

	related, err := h.Service.GetRelatedReturns(datasetID, returnID, queryInt(r, "limit", 8))
	if err != nil {
		writeLookupError(w, err, "dataset %s or return %s not found", datasetID, returnID)
		return
	}

	writeJSON(w, http.StatusOK, related)
}

func (h *Handler) DatasetReturnGraphHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID := vars["datasetId"]
	returnID := vars["returnId"]
	if datasetID == "" || returnID == "" {
		writeError(w, http.StatusBadRequest, "dataset id and return id are required")
		return
	}

	graph, err := h.Service.GetGraphProjection(datasetID, returnID, queryInt(r, "limit", 24))
	if err != nil {
		writeLookupError(w, err, "dataset %s or return %s not found", datasetID, returnID)
		return
	}

	writeJSON(w, http.StatusOK, graph)
}

func (h *Handler) DatasetReturnFeaturesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID := vars["datasetId"]
	returnID := vars["returnId"]
	if datasetID == "" {
		writeError(w, http.StatusBadRequest, "dataset id is required")
		return
	}
	if returnID == "" {
		writeError(w, http.StatusBadRequest, "return id is required")
		return
	}

	features, err := h.Service.GetReturnFeaturesForDataset(datasetID, returnID)
	if err != nil {
		if errors.Is(err, service.ErrReturnNotFound) {
			writeError(w, http.StatusNotFound, "return %s not found in dataset %s", returnID, datasetID)
			return
		}
		h.Logger.Error("dataset return features error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get return features: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, features)
}

func (h *Handler) DatasetScoringInputsHandler(w http.ResponseWriter, r *http.Request) {
	datasetID := mux.Vars(r)["datasetId"]
	if datasetID == "" {
		writeError(w, http.StatusBadRequest, "dataset id is required")
		return
	}

	inputs, err := h.Service.GetScoringInputs(datasetID)
	if err != nil {
		writeLookupError(w, err, "dataset %s not found", datasetID)
		return
	}

	writeJSON(w, http.StatusOK, inputs)
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	writeJSON(w, status, response{Message: message})
}

func writeLookupError(w http.ResponseWriter, err error, format string, args ...interface{}) {
	if errors.Is(err, service.ErrDatasetNotFound) || errors.Is(err, service.ErrReturnNotFound) {
		writeError(w, http.StatusNotFound, format, args...)
		return
	}
	writeError(w, http.StatusInternalServerError, "relations service error: %v", err)
}

func queryInt(r *http.Request, key string, fallback int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
