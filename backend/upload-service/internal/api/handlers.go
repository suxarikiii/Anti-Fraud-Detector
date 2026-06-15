package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"upload-service/internal/service"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type Handler struct {
	Service *service.Service
	Logger  *slog.Logger
}

func NewHandler(service *service.Service, logger *slog.Logger) *Handler {
	return &Handler{Service: service, Logger: logger}
}

type response struct {
	Message   string      `json:"message,omitempty"`
	DatasetID string      `json:"datasetId,omitempty"`
	JobID     string      `json:"jobId,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Filename  string      `json:"filename,omitempty"`
	Status    string      `json:"status,omitempty"`
}

func (h *Handler) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, response{Data: map[string]string{"status": "UP", "service": "upload-service"}})
}

func (h *Handler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file: %v", err)
		return
	}
	defer file.Close()

	buffer, err := readFile(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read upload: %v", err)
		return
	}

	datasetID, jobID, err := h.Service.UploadDataset(r.Context(), bytes.NewReader(buffer), int64(len(buffer)), header.Filename)
	if err != nil {
		h.Logger.Error("upload service error", "error", err)
		writeError(w, http.StatusInternalServerError, "upload error: %v", err)
		return
	}

	writeJSON(w, http.StatusCreated, response{DatasetID: datasetID.String(), JobID: jobID.String(), Filename: header.Filename, Status: "UPLOADED"})
}

func (h *Handler) PreviewHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID, err := uuid.Parse(vars["datasetId"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dataset id")
		return
	}

	preview, err := h.Service.PreviewDataset(r.Context(), datasetID)
	if err != nil {
		h.Logger.Error("preview error", "error", err)
		writeError(w, http.StatusNotFound, "preview not available: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, response{Data: preview})
}

func (h *Handler) StartAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID, err := uuid.Parse(vars["datasetId"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dataset id")
		return
	}

	jobID, err := h.Service.StartAnalysis(r.Context(), datasetID)
	if err != nil {
		h.Logger.Error("start analysis error", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to start analysis: %v", err)
		return
	}

	writeJSON(w, http.StatusCreated, response{JobID: jobID.String()})
}

func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID, err := uuid.Parse(vars["jobId"])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	status, err := h.Service.GetAnalysisStatus(r.Context(), jobID)
	if err != nil {
		h.Logger.Error("status lookup error", "error", err)
		writeError(w, http.StatusNotFound, "job not found: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, response{Data: status})
}

func readFile(file multipart.File) ([]byte, error) {
	return io.ReadAll(file)
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
