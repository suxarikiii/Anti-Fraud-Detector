package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"

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

type errorResponse struct {
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Path      string `json:"path"`
	Timestamp string `json:"timestamp"`
}

type updateStatusRequest struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (h *Handler) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP", "service": "upload-service"})
}

func (h *Handler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_UPLOAD", "CSV file field %q is required", "file")
		return
	}
	defer file.Close()

	buffer, err := readFile(file)
	if err != nil {
		h.Logger.Error("failed to read upload", "error", err)
		writeError(w, r, http.StatusInternalServerError, "UPLOAD_READ_FAILED", "failed to read uploaded file")
		return
	}

	datasetID, jobID, err := h.Service.UploadDataset(r.Context(), bytes.NewReader(buffer), int64(len(buffer)), header.Filename)
	if err != nil {
		var invalidUpload *service.InvalidUploadError
		if errors.As(err, &invalidUpload) {
			writeError(w, r, http.StatusBadRequest, "INVALID_CSV", invalidUpload.Message)
			return
		}
		h.Logger.Error("upload service error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "UPLOAD_FAILED", "failed to upload dataset")
		return
	}

	writeJSON(w, http.StatusCreated, response{DatasetID: datasetID.String(), JobID: jobID.String(), Filename: header.Filename, Status: "UPLOADED"})
}

func (h *Handler) PreviewHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID, err := uuid.Parse(vars["datasetId"])
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_DATASET_ID", "invalid dataset id")
		return
	}

	preview, err := h.Service.PreviewDataset(r.Context(), datasetID)
	if err != nil {
		var notFound *service.NotFoundError
		if errors.As(err, &notFound) {
			writeError(w, r, http.StatusNotFound, "DATASET_NOT_FOUND", notFound.Error())
			return
		}
		h.Logger.Error("preview error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "PREVIEW_FAILED", "failed to load dataset preview")
		return
	}

	writeJSON(w, http.StatusOK, preview)
}

func (h *Handler) StartAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	datasetID, err := uuid.Parse(vars["datasetId"])
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_DATASET_ID", "invalid dataset id")
		return
	}

	jobID, err := h.Service.StartAnalysis(r.Context(), datasetID)
	if err != nil {
		var notFound *service.NotFoundError
		if errors.As(err, &notFound) {
			writeError(w, r, http.StatusNotFound, "DATASET_NOT_FOUND", notFound.Error())
			return
		}
		h.Logger.Error("start analysis error", "error", err)
		writeError(w, r, http.StatusBadGateway, "ANALYSIS_START_FAILED", "failed to start analysis; job was marked FAILED when possible")
		return
	}

	statusValue := "NORMALIZING"
	if status, statusErr := h.Service.GetAnalysisStatus(r.Context(), jobID); statusErr == nil {
		statusValue = status.Status
	}
	writeJSON(w, http.StatusCreated, response{JobID: jobID.String(), Status: statusValue})
}

func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID, err := uuid.Parse(vars["jobId"])
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JOB_ID", "invalid job id")
		return
	}

	status, err := h.Service.GetAnalysisStatus(r.Context(), jobID)
	if err != nil {
		var notFound *service.NotFoundError
		if errors.As(err, &notFound) {
			writeError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", notFound.Error())
			return
		}
		h.Logger.Error("status lookup error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "STATUS_LOOKUP_FAILED", "failed to load analysis status")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) UpdateStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID, err := uuid.Parse(vars["jobId"])
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JOB_ID", "invalid job id")
		return
	}

	var request updateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_STATUS_BODY", "invalid status update body")
		return
	}

	status, err := h.Service.UpdateAnalysisStatus(r.Context(), jobID, request.Status, request.ErrorMessage)
	if err != nil {
		var invalidStatus *service.InvalidUploadError
		if errors.As(err, &invalidStatus) {
			writeError(w, r, http.StatusBadRequest, "INVALID_ANALYSIS_STATUS", invalidStatus.Message)
			return
		}
		var notFound *service.NotFoundError
		if errors.As(err, &notFound) {
			writeError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", notFound.Error())
			return
		}
		h.Logger.Error("status update error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "STATUS_UPDATE_FAILED", "failed to update analysis status")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func readFile(file multipart.File) ([]byte, error) {
	return io.ReadAll(file)
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	writeJSON(w, status, errorResponse{
		Status:    status,
		Error:     http.StatusText(status),
		Code:      code,
		Message:   message,
		Path:      r.URL.Path,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
