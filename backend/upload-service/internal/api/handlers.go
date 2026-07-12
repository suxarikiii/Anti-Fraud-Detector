package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
	Message   string                    `json:"message,omitempty"`
	DatasetID string                    `json:"datasetId,omitempty"`
	JobID     string                    `json:"jobId,omitempty"`
	Data      interface{}               `json:"data,omitempty"`
	Filename  string                    `json:"filename,omitempty"`
	Status    string                    `json:"status,omitempty"`
	RowCount  int                       `json:"rowCount,omitempty"`
	SizeBytes int64                     `json:"sizeBytes,omitempty"`
	Warnings  []service.ValidationIssue `json:"warnings,omitempty"`
}

type errorResponse struct {
	Status    int                       `json:"status"`
	Error     string                    `json:"error"`
	Code      string                    `json:"code"`
	Message   string                    `json:"message"`
	Path      string                    `json:"path"`
	Timestamp string                    `json:"timestamp"`
	Errors    []service.ValidationIssue `json:"errors,omitempty"`
	Warnings  []service.ValidationIssue `json:"warnings,omitempty"`
}

type updateStatusRequest struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (h *Handler) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP", "service": "upload-service"})
}

func (h *Handler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	// The multipart envelope needs a small allowance in addition to the file limit.
	r.Body = http.MaxBytesReader(w, r.Body, h.Service.MaxFileSize()+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(strings.ToLower(err.Error()), "too large") {
			writeError(w, r, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "uploaded file exceeds the configured size limit")
			return
		}
		writeError(w, r, http.StatusBadRequest, "INVALID_UPLOAD", "invalid multipart upload")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_UPLOAD", "CSV file field %q is required", "file")
		return
	}
	defer file.Close()

	result, err := h.Service.UploadDatasetDetailed(r.Context(), file, header.Size, header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		var invalidUpload *service.InvalidUploadError
		if errors.As(err, &invalidUpload) {
			writeValidationError(w, r, http.StatusBadRequest, "INVALID_CSV", invalidUpload)
			return
		}
		h.Logger.Error("upload service error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "UPLOAD_FAILED", "failed to upload dataset")
		return
	}

	writeJSON(w, http.StatusCreated, response{DatasetID: result.DatasetID.String(), JobID: result.JobID.String(), Filename: result.Filename, Status: "UPLOADED", RowCount: result.RowCount, SizeBytes: result.SizeBytes, Warnings: result.Warnings})
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
		var invalid *service.InvalidUploadError
		if errors.As(err, &invalid) {
			writeError(w, r, http.StatusConflict, "ANALYSIS_START_NOT_ALLOWED", invalid.Message)
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

func (h *Handler) ListDatasetsHandler(w http.ResponseWriter, r *http.Request) {
	page, err := positiveQueryInt(r, "page", 1)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PAGE", err.Error())
		return
	}
	pageSize, err := positiveQueryInt(r, "pageSize", 20)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_PAGE_SIZE", err.Error())
		return
	}
	from, err := optionalQueryTime(r, "from")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_DATE_FILTER", err.Error())
		return
	}
	to, err := optionalQueryTime(r, "to")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_DATE_FILTER", err.Error())
		return
	}
	result, err := h.Service.ListDatasets(r.Context(), r.URL.Query().Get("status"), from, to, page, pageSize)
	if err != nil {
		var invalid *service.InvalidUploadError
		if errors.As(err, &invalid) {
			writeError(w, r, http.StatusBadRequest, "INVALID_FILTER", invalid.Message)
			return
		}
		h.Logger.Error("dataset list error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "DATASET_LIST_FAILED", "failed to list datasets")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) DatasetDetailsHandler(w http.ResponseWriter, r *http.Request) {
	datasetID, err := uuid.Parse(mux.Vars(r)["datasetId"])
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_DATASET_ID", "invalid dataset id")
		return
	}
	result, err := h.Service.GetDatasetDetails(r.Context(), datasetID)
	if err != nil {
		var notFound *service.NotFoundError
		if errors.As(err, &notFound) {
			writeError(w, r, http.StatusNotFound, "DATASET_NOT_FOUND", notFound.Error())
			return
		}
		h.Logger.Error("dataset details error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "DATASET_DETAILS_FAILED", "failed to load dataset details")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) RetryAnalysisHandler(w http.ResponseWriter, r *http.Request) {
	jobID, err := uuid.Parse(mux.Vars(r)["jobId"])
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JOB_ID", "invalid job id")
		return
	}
	retryJobID, err := h.Service.RetryAnalysis(r.Context(), jobID)
	if err != nil {
		var notFound *service.NotFoundError
		if errors.As(err, &notFound) {
			writeError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", notFound.Error())
			return
		}
		var invalid *service.InvalidUploadError
		if errors.As(err, &invalid) {
			writeError(w, r, http.StatusConflict, "RETRY_NOT_ALLOWED", invalid.Message)
			return
		}
		h.Logger.Error("analysis retry error", "error", err)
		writeError(w, r, http.StatusBadGateway, "ANALYSIS_RETRY_FAILED", "failed to retry analysis")
		return
	}
	status := domainStatus(h.Service, r, retryJobID)
	writeJSON(w, http.StatusAccepted, response{JobID: retryJobID.String(), Status: status})
}

func (h *Handler) ArchiveDatasetHandler(w http.ResponseWriter, r *http.Request) {
	datasetID, err := uuid.Parse(mux.Vars(r)["datasetId"])
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_DATASET_ID", "invalid dataset id")
		return
	}
	if err = h.Service.ArchiveDataset(r.Context(), datasetID); err != nil {
		var notFound *service.NotFoundError
		if errors.As(err, &notFound) {
			writeError(w, r, http.StatusNotFound, "DATASET_NOT_FOUND", notFound.Error())
			return
		}
		var invalid *service.InvalidUploadError
		if errors.As(err, &invalid) {
			writeError(w, r, http.StatusConflict, "ARCHIVE_NOT_ALLOWED", invalid.Message)
			return
		}
		h.Logger.Error("dataset archive error", "error", err)
		writeError(w, r, http.StatusInternalServerError, "DATASET_ARCHIVE_FAILED", "failed to archive dataset")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func domainStatus(svc *service.Service, r *http.Request, jobID uuid.UUID) string {
	status, err := svc.GetAnalysisStatus(r.Context(), jobID)
	if err != nil {
		return "UPLOADED"
	}
	return status.Status
}

func positiveQueryInt(r *http.Request, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func optionalQueryTime(r *http.Request, name string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be an RFC3339 timestamp", name)
	}
	value = value.UTC()
	return &value, nil
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, format string, args ...interface{}) {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}
	writeJSON(w, status, errorResponse{
		Status:    status,
		Error:     http.StatusText(status),
		Code:      code,
		Message:   message,
		Path:      r.URL.Path,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func writeValidationError(w http.ResponseWriter, r *http.Request, status int, code string, validation *service.InvalidUploadError) {
	writeJSON(w, status, errorResponse{Status: status, Error: http.StatusText(status), Code: code,
		Message: validation.Message, Path: r.URL.Path, Timestamp: time.Now().UTC().Format(time.RFC3339),
		Errors: validation.Errors, Warnings: validation.Warnings})
}
