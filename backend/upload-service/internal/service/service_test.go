package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"upload-service/internal/domain"

	"github.com/google/uuid"
)

const cleanRefundCSV = `order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,product_category,return_reason,evidence_provided,decision,manual_override,decision_time_minutes,timestamp
order_1001,customer_200,return_3001,agent_001,203.84,199.57,clothing,changed_mind,True,APPROVED,False,64,2026-06-01T09:06:00Z
order_1002,customer_201,return_3002,agent_005,143.16,131.27,sports,item_not_as_described,True,APPROVED,False,35,2026-06-01T09:22:00Z
`

const dirtyRefundCSV = `purchase_id,buyer_id,refund_request_id,agent_id,purchase_amount,return_amount,category,reason,has_photo,status,override,resolution_minutes,created_at
purchase_1001,buyer_200,refund_req_3001,agent_001,203.84,199.57,clothing,changed_mind,yes,approved,no,64,2026-06-01 09:06:00
purchase_1002,buyer_201,refund_req_3002,agent_005,143.16,131.27,sports,item_not_as_described,yes,approved,no,35,2026-06-01 09:22:00
`

func TestParseCSVPreviewCleanAndDirtyRefundFiles(t *testing.T) {
	tests := []struct {
		name        string
		csv         string
		firstHeader string
		firstCell   string
	}{
		{name: "clean", csv: cleanRefundCSV, firstHeader: "order_id", firstCell: "order_1001"},
		{name: "dirty", csv: dirtyRefundCSV, firstHeader: "purchase_id", firstCell: "purchase_1001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preview, err := parseCSVPreview(strings.NewReader(tt.csv), 20)
			if err != nil {
				t.Fatalf("parse preview: %v", err)
			}

			if got := preview.Headers[0]; got != tt.firstHeader {
				t.Fatalf("first header = %q, want %q", got, tt.firstHeader)
			}
			if got := preview.Rows[0][0]; got != tt.firstCell {
				t.Fatalf("first row cell = %q, want %q", got, tt.firstCell)
			}
			if len(preview.Rows) != 2 {
				t.Fatalf("rows = %d, want 2", len(preview.Rows))
			}
		})
	}
}

func TestValidateUploadedCSVInvalidCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		body     string
		want     string
	}{
		{name: "non csv extension", filename: "refunds.txt", body: cleanRefundCSV, want: ".csv"},
		{name: "empty file", filename: "refunds.csv", body: "  \n", want: "empty"},
		{name: "missing required header", filename: "refunds.csv", body: "order_id,customer_id\norder_1,customer_1\n", want: "missing required column"},
		{name: "invalid row width", filename: "refunds.csv", body: "order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,decision,timestamp\norder_1,customer_1\n", want: "invalid CSV row"},
		{name: "invalid numeric value", filename: "refunds.csv", body: "order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,decision,timestamp\norder_1,customer_1,return_1,agent_1,nope,9.99,APPROVED,2026-06-01T09:06:00Z\n", want: "invalid numeric value"},
		{name: "no rows", filename: "refunds.csv", body: "order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,decision,timestamp\n", want: "at least one data row"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUploadedCSV(tt.filename, []byte(tt.body))
			if err == nil {
				t.Fatal("expected validation error")
			}

			var invalid *InvalidUploadError
			if !errors.As(err, &invalid) {
				t.Fatalf("error type = %T, want InvalidUploadError", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestUploadPreviewStartStatusFlow(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	publisher := &fakePublisher{}
	svc := NewServiceWithStore(repo, store, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))

	datasetID, uploadJobID, err := svc.UploadDataset(ctx, strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "clean_refund_dataset.csv")
	if err != nil {
		t.Fatalf("upload dataset: %v", err)
	}

	preview, err := svc.PreviewDataset(ctx, datasetID)
	if err != nil {
		t.Fatalf("preview dataset: %v", err)
	}
	if got := preview.Headers[0]; got != "order_id" {
		t.Fatalf("preview first header = %q, want order_id", got)
	}
	if got := preview.Rows[0][0]; got != "order_1001" {
		t.Fatalf("preview first row = %q, want order_1001", got)
	}

	jobID, err := svc.StartAnalysis(ctx, datasetID)
	if err != nil {
		t.Fatalf("start analysis: %v", err)
	}
	if jobID != uploadJobID {
		t.Fatalf("start job id = %s, want uploaded job id %s", jobID, uploadJobID)
	}

	status, err := svc.GetAnalysisStatus(ctx, jobID)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.Status != domain.AnalysisStatusNormalizing {
		t.Fatalf("status = %q, want %q", status.Status, domain.AnalysisStatusNormalizing)
	}
	if status.ProgressPercent != 20 {
		t.Fatalf("progress = %d, want 20", status.ProgressPercent)
	}
	if !strings.Contains(status.Message, "Normalizing") {
		t.Fatalf("message = %q, want dashboard-friendly normalizing message", status.Message)
	}

	if len(publisher.messages) != 1 {
		t.Fatalf("published messages = %d, want 1", len(publisher.messages))
	}
	if got := publisher.messages[0].routingKey; got != datasetUploadedRoutingKey {
		t.Fatalf("routing key = %q, want %q", got, datasetUploadedRoutingKey)
	}

	event, ok := publisher.messages[0].payload.(datasetUploadedEvent)
	if !ok {
		t.Fatalf("payload type = %T, want datasetUploadedEvent", publisher.messages[0].payload)
	}
	if event.DatasetID != datasetID.String() || event.JobID != jobID.String() {
		t.Fatalf("event ids = dataset %s job %s, want dataset %s job %s", event.DatasetID, event.JobID, datasetID, jobID)
	}
	if event.FilePath == "" || event.Timestamp == "" {
		body, _ := json.Marshal(event)
		t.Fatalf("event missing file path or timestamp: %s", body)
	}
}

func TestUpdateAnalysisStatus(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	svc := NewServiceWithStore(repo, store, &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, jobID, err := svc.UploadDataset(ctx, strings.NewReader(dirtyRefundCSV), int64(len(dirtyRefundCSV)), "dirty_business_refund_dataset.csv")
	if err != nil {
		t.Fatalf("upload dataset: %v", err)
	}

	status, err := svc.UpdateAnalysisStatus(ctx, jobID, domain.AnalysisStatusScoring, "")
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if status.Status != domain.AnalysisStatusScoring || status.ProgressPercent != 80 {
		t.Fatalf("status = %s progress = %d, want SCORING/80", status.Status, status.ProgressPercent)
	}
}

type fakeMessage struct {
	routingKey string
	payload    interface{}
}

type fakePublisher struct {
	messages []fakeMessage
}

func (p *fakePublisher) Publish(_ context.Context, routingKey string, payload interface{}) error {
	p.messages = append(p.messages, fakeMessage{routingKey: routingKey, payload: payload})
	return nil
}

type fakeStore struct {
	objects map[string][]byte
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: make(map[string][]byte)}
}

func (s *fakeStore) Put(_ context.Context, objectName string, file io.Reader, _ int64, _ string) error {
	body, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	s.objects[objectName] = body
	return nil
}

func (s *fakeStore) Get(_ context.Context, objectName string) (io.ReadCloser, error) {
	body, ok := s.objects[objectName]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

type fakeRepo struct {
	files map[uuid.UUID]*domain.UploadedFile
	jobs  map[uuid.UUID]*domain.AnalysisJob
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		files: make(map[uuid.UUID]*domain.UploadedFile),
		jobs:  make(map[uuid.UUID]*domain.AnalysisJob),
	}
}

func (r *fakeRepo) CreateDatasetWithFile(_ context.Context, datasetID uuid.UUID, _, _, _, filePath, fileType string, uploadedAt, _ time.Time) error {
	r.files[datasetID] = &domain.UploadedFile{
		ID:         uuid.New(),
		DatasetID:  datasetID,
		FilePath:   filePath,
		FileType:   fileType,
		UploadedAt: uploadedAt,
	}
	return nil
}

func (r *fakeRepo) GetUploadedFileByDatasetID(_ context.Context, datasetID uuid.UUID) (*domain.UploadedFile, error) {
	file, ok := r.files[datasetID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return file, nil
}

func (r *fakeRepo) CreateAnalysisJob(_ context.Context, jobID, datasetID uuid.UUID, status, currentStep string, createdAt, updatedAt time.Time) error {
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

func (r *fakeRepo) GetAnalysisJobByID(_ context.Context, jobID uuid.UUID) (*domain.AnalysisJob, error) {
	job, ok := r.jobs[jobID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	copy := *job
	return &copy, nil
}

func (r *fakeRepo) GetLatestAnalysisJobByDatasetID(_ context.Context, datasetID uuid.UUID) (*domain.AnalysisJob, error) {
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

func (r *fakeRepo) UpdateAnalysisStatus(_ context.Context, jobID uuid.UUID, status, currentStep string, updatedAt time.Time) error {
	return r.UpdateAnalysisStatusWithError(context.Background(), jobID, status, currentStep, "", updatedAt)
}

func (r *fakeRepo) UpdateAnalysisStatusWithError(_ context.Context, jobID uuid.UUID, status, currentStep, errorMessage string, updatedAt time.Time) error {
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
