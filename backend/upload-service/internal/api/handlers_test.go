package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"upload-service/internal/domain"
	"upload-service/internal/service"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func TestUploadHandlerEmptyCSVReturnsStructuredError(t *testing.T) {
	handler := newTestHandler()
	body, contentType := multipartUploadBody(t, "empty.csv", " \n")
	request := httptest.NewRequest(http.MethodPost, "/api/datasets/upload", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	handler.UploadHandler(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	response := decodeErrorResponse(t, recorder.Body)
	if response.Code != "INVALID_CSV" {
		t.Fatalf("code = %q, want INVALID_CSV", response.Code)
	}
	if response.Message != "uploaded CSV is empty" {
		t.Fatalf("message = %q, want empty CSV message", response.Message)
	}
	if response.Path != "/api/datasets/upload" {
		t.Fatalf("path = %q, want upload path", response.Path)
	}
}

func TestPreviewHandlerDatasetNotFoundReturnsStructuredError(t *testing.T) {
	handler := newTestHandler()
	datasetID := uuid.New().String()
	request := httptest.NewRequest(http.MethodGet, "/api/datasets/"+datasetID+"/preview", nil)
	request = mux.SetURLVars(request, map[string]string{"datasetId": datasetID})
	recorder := httptest.NewRecorder()

	handler.PreviewHandler(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	response := decodeErrorResponse(t, recorder.Body)
	if response.Code != "DATASET_NOT_FOUND" {
		t.Fatalf("code = %q, want DATASET_NOT_FOUND", response.Code)
	}
	if response.Status != http.StatusNotFound {
		t.Fatalf("json status = %d, want %d", response.Status, http.StatusNotFound)
	}
}

func TestStatusHandlerJobNotFoundReturnsStructuredError(t *testing.T) {
	handler := newTestHandler()
	jobID := uuid.New().String()
	request := httptest.NewRequest(http.MethodGet, "/api/analysis/"+jobID+"/status", nil)
	request = mux.SetURLVars(request, map[string]string{"jobId": jobID})
	recorder := httptest.NewRecorder()

	handler.StatusHandler(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	response := decodeErrorResponse(t, recorder.Body)
	if response.Code != "JOB_NOT_FOUND" {
		t.Fatalf("code = %q, want JOB_NOT_FOUND", response.Code)
	}
}

func newTestHandler() *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewServiceWithStore(newAPIFakeRepo(), newAPIFakeStore(), &apiFakePublisher{}, logger)
	return NewHandler(svc, logger)
}

func multipartUploadBody(t *testing.T, filename, content string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return body, writer.FormDataContentType()
}

func decodeErrorResponse(t *testing.T, reader io.Reader) errorResponse {
	t.Helper()

	var response errorResponse
	if err := json.NewDecoder(reader).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return response
}

type apiFakePublisher struct{}

func (p *apiFakePublisher) Publish(_ context.Context, _ string, _ interface{}) error {
	return nil
}

type apiFakeStore struct {
	objects map[string][]byte
}

func newAPIFakeStore() *apiFakeStore {
	return &apiFakeStore{objects: make(map[string][]byte)}
}

func (s *apiFakeStore) Put(_ context.Context, objectName string, file io.Reader, _ int64, _ string) error {
	body, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	s.objects[objectName] = body
	return nil
}

func (s *apiFakeStore) Get(_ context.Context, objectName string) (io.ReadCloser, error) {
	body, ok := s.objects[objectName]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

type apiFakeRepo struct {
	files map[uuid.UUID]*domain.UploadedFile
	jobs  map[uuid.UUID]*domain.AnalysisJob
}

func newAPIFakeRepo() *apiFakeRepo {
	return &apiFakeRepo{
		files: make(map[uuid.UUID]*domain.UploadedFile),
		jobs:  make(map[uuid.UUID]*domain.AnalysisJob),
	}
}

func (r *apiFakeRepo) CreateDatasetWithFile(_ context.Context, datasetID uuid.UUID, _, _, _, filePath, fileType string, uploadedAt, _ time.Time) error {
	r.files[datasetID] = &domain.UploadedFile{
		ID:         uuid.New(),
		DatasetID:  datasetID,
		FilePath:   filePath,
		FileType:   fileType,
		UploadedAt: uploadedAt,
	}
	return nil
}

func (r *apiFakeRepo) GetUploadedFileByDatasetID(_ context.Context, datasetID uuid.UUID) (*domain.UploadedFile, error) {
	file, ok := r.files[datasetID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return file, nil
}

func (r *apiFakeRepo) CreateAnalysisJob(_ context.Context, jobID, datasetID uuid.UUID, status, currentStep string, createdAt, updatedAt time.Time) error {
	r.jobs[jobID] = &domain.AnalysisJob{
		ID:          jobID,
		DatasetID:   datasetID,
		Status:      status,
		CurrentStep: currentStep,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	return nil
}

func (r *apiFakeRepo) GetAnalysisJobByID(_ context.Context, jobID uuid.UUID) (*domain.AnalysisJob, error) {
	job, ok := r.jobs[jobID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	copy := *job
	return &copy, nil
}

func (r *apiFakeRepo) GetLatestAnalysisJobByDatasetID(_ context.Context, datasetID uuid.UUID) (*domain.AnalysisJob, error) {
	var latest *domain.AnalysisJob
	for _, job := range r.jobs {
		if job.DatasetID != datasetID {
			continue
		}
		if latest == nil || job.CreatedAt.After(latest.CreatedAt) {
			latest = job
		}
	}
	if latest == nil {
		return nil, sql.ErrNoRows
	}
	copy := *latest
	return &copy, nil
}

func (r *apiFakeRepo) UpdateAnalysisStatus(_ context.Context, jobID uuid.UUID, status, currentStep string, updatedAt time.Time) error {
	return r.UpdateAnalysisStatusWithError(context.Background(), jobID, status, currentStep, "", updatedAt)
}

func (r *apiFakeRepo) UpdateAnalysisStatusWithError(_ context.Context, jobID uuid.UUID, status, currentStep, errorMessage string, updatedAt time.Time) error {
	job, ok := r.jobs[jobID]
	if !ok {
		return sql.ErrNoRows
	}
	job.Status = status
	job.CurrentStep = currentStep
	job.Error = errorMessage
	job.UpdatedAt = updatedAt
	return nil
}
