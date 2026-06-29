package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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

func (h *Handler) CustomerHistoryHandler(w http.ResponseWriter, r *http.Request) {
	customerID := mux.Vars(r)["customerId"]
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "customer id is required")
		return
	}

	writeJSON(w, http.StatusOK, h.Service.GetCustomerHistory(customerID))
}

func (h *Handler) AgentSummaryHandler(w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agentId"]
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	writeJSON(w, http.StatusOK, h.Service.GetAgentSummary(agentID))
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

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	writeJSON(w, status, response{Message: message})
}
