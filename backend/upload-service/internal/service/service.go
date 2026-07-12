package service

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"upload-service/internal/domain"
	"upload-service/internal/repository"
	"upload-service/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

const (
	datasetUploadedRoutingKey  = "dataset.uploaded"
	maxPreviewRows             = 20
	defaultMaxFileSize         = 50 << 20
	defaultMaxRows             = 250_000
	defaultMaxValidationErrors = 100
)

type PreviewResponse struct {
	Headers   []string            `json:"headers"`
	Rows      []map[string]string `json:"rows"`
	RawRows   [][]string          `json:"rawRows"`
	RowCount  int                 `json:"rowCount"`
	Truncated bool                `json:"truncated"`
}

type StatusStage struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	State   string `json:"state"`
}

type AnalysisStatusResponse struct {
	ID              string        `json:"id"`
	JobID           string        `json:"jobId"`
	DatasetID       string        `json:"datasetId"`
	Status          string        `json:"status"`
	CurrentStep     string        `json:"currentStep"`
	Message         string        `json:"message"`
	ProgressPercent int           `json:"progressPercent"`
	Stages          []StatusStage `json:"stages"`
	Error           string        `json:"errorMessage,omitempty"`
	FailedStage     string        `json:"failedStage,omitempty"`
	ResultReady     bool          `json:"resultReady"`
	RetryOfJobID    string        `json:"retryOfJobId,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
	StartedAt       *time.Time    `json:"startedAt,omitempty"`
	CompletedAt     *time.Time    `json:"completedAt,omitempty"`
	FailedAt        *time.Time    `json:"failedAt,omitempty"`
}

type InvalidUploadError struct {
	Message  string
	Errors   []ValidationIssue
	Warnings []ValidationIssue
}

func (e *InvalidUploadError) Error() string {
	return e.Message
}

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

type DatasetRepository interface {
	CreateDatasetWithFile(ctx context.Context, datasetID uuid.UUID, name, originalFilename, status, filePath, fileType string, uploadedAt, createdAt time.Time) error
	GetUploadedFileByDatasetID(ctx context.Context, datasetID uuid.UUID) (*domain.UploadedFile, error)
	CreateAnalysisJob(ctx context.Context, jobID, datasetID uuid.UUID, status, currentStep string, createdAt, updatedAt time.Time) error
	GetAnalysisJobByID(ctx context.Context, jobID uuid.UUID) (*domain.AnalysisJob, error)
	GetLatestAnalysisJobByDatasetID(ctx context.Context, datasetID uuid.UUID) (*domain.AnalysisJob, error)
	UpdateAnalysisStatus(ctx context.Context, jobID uuid.UUID, status, currentStep string, updatedAt time.Time) error
	UpdateAnalysisStatusWithError(ctx context.Context, jobID uuid.UUID, status, currentStep, errorMessage string, updatedAt time.Time) error
}

type Publisher interface {
	Publish(ctx context.Context, routingKey string, payload interface{}) error
}

type ObjectStore interface {
	Put(ctx context.Context, objectName string, file io.Reader, size int64, contentType string) error
	Get(ctx context.Context, objectName string) (io.ReadCloser, error)
}

type minioObjectStore struct {
	client *minio.Client
	bucket string
}

func (s *minioObjectStore) Put(ctx context.Context, objectName string, file io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, objectName, file, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *minioObjectStore) Get(ctx context.Context, objectName string) (io.ReadCloser, error) {
	return s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
}

func (s *minioObjectStore) Delete(ctx context.Context, objectName string) error {
	return s.client.RemoveObject(ctx, s.bucket, objectName, minio.RemoveObjectOptions{})
}

type Service struct {
	repo        DatasetRepository
	store       ObjectStore
	publisher   Publisher
	logger      *slog.Logger
	maxFileSize int64
	maxRows     int
	maxErrors   int
}

type datasetUploadedEvent struct {
	DatasetID  string `json:"datasetId"`
	JobID      string `json:"jobId"`
	Filename   string `json:"filename"`
	FilePath   string `json:"filePath"`
	FileType   string `json:"fileType"`
	UploadedAt string `json:"uploadedAt"`
	Timestamp  string `json:"timestamp"`
}

func NewService(repo *repository.Repository, minioClient *minio.Client, bucket string, publisher *rabbitmq.Publisher, logger *slog.Logger) *Service {
	return NewServiceWithStore(repo, &minioObjectStore{client: minioClient, bucket: bucket}, publisher, logger)
}

func NewServiceWithStore(repo DatasetRepository, store ObjectStore, publisher Publisher, logger *slog.Logger) *Service {
	return &Service{
		repo: repo, store: store, publisher: publisher, logger: logger,
		maxFileSize: defaultMaxFileSize, maxRows: defaultMaxRows, maxErrors: defaultMaxValidationErrors,
	}
}

func (s *Service) ConfigureUploadLimits(maxFileSize int64, maxRows, maxErrors int) {
	if maxFileSize > 0 {
		s.maxFileSize = maxFileSize
	}
	if maxRows > 0 {
		s.maxRows = maxRows
	}
	if maxErrors > 0 {
		s.maxErrors = maxErrors
	}
}

func (s *Service) MaxFileSize() int64 { return s.maxFileSize }

func (s *Service) UploadDataset(ctx context.Context, file io.Reader, size int64, originalFilename string) (uuid.UUID, uuid.UUID, error) {
	result, err := s.UploadDatasetDetailed(ctx, file, size, originalFilename, "text/csv")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return result.DatasetID, result.JobID, nil
}

func (s *Service) PreviewDataset(ctx context.Context, datasetID uuid.UUID) (*PreviewResponse, error) {
	uploadedFile, err := s.repo.GetUploadedFileByDatasetID(ctx, datasetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &NotFoundError{Resource: "dataset", ID: datasetID.String()}
		}
		return nil, err
	}

	object, err := s.store.Get(ctx, uploadedFile.FilePath)
	if err != nil {
		return nil, fmt.Errorf("minio get object: %w", err)
	}
	defer object.Close()

	return parseCSVPreview(object, maxPreviewRows)
}

func (s *Service) StartAnalysis(ctx context.Context, datasetID uuid.UUID) (uuid.UUID, error) {
	uploadedFile, err := s.repo.GetUploadedFileByDatasetID(ctx, datasetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, &NotFoundError{Resource: "dataset", ID: datasetID.String()}
		}
		return uuid.Nil, err
	}

	now := time.Now().UTC()

	job, err := s.repo.GetLatestAnalysisJobByDatasetID(ctx, datasetID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("get analysis job: %w", err)
		}

		jobID := uuid.New()
		if err := s.repo.CreateAnalysisJob(ctx, jobID, datasetID, domain.AnalysisStatusUploaded, domain.AnalysisStatusUploaded, now, now); err != nil {
			return uuid.Nil, fmt.Errorf("create analysis job: %w", err)
		}

		job, err = s.repo.GetAnalysisJobByID(ctx, jobID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("load analysis job: %w", err)
		}
	}

	if job.Status != domain.AnalysisStatusUploaded {
		return job.ID, nil
	}
	return s.startAnalysisJob(ctx, uploadedFile, job, now)
}

func (s *Service) startAnalysisJob(ctx context.Context, uploadedFile *domain.UploadedFile, job *domain.AnalysisJob, now time.Time) (uuid.UUID, error) {
	if job.Status != domain.AnalysisStatusUploaded {
		return job.ID, nil
	}

	claimed := true
	var err error
	if repo, ok := s.repo.(interface {
		ClaimAnalysisStart(context.Context, uuid.UUID, time.Time) (bool, error)
	}); ok {
		claimed, err = repo.ClaimAnalysisStart(ctx, job.ID, now)
		if errors.Is(err, repository.ErrDatasetArchived) {
			return uuid.Nil, &InvalidUploadError{Message: repository.ErrDatasetArchived.Error()}
		}
		if err != nil {
			return uuid.Nil, fmt.Errorf("claim analysis start: %w", err)
		}
		if !claimed {
			return job.ID, nil
		}
	}

	event := datasetUploadedEvent{
		DatasetID:  job.DatasetID.String(),
		JobID:      job.ID.String(),
		Filename:   filepath.Base(uploadedFile.FilePath),
		FilePath:   uploadedFile.FilePath,
		FileType:   uploadedFile.FileType,
		UploadedAt: uploadedFile.UploadedAt.UTC().Format(time.RFC3339),
		Timestamp:  now.Format(time.RFC3339),
	}

	if err := s.publisher.Publish(ctx, datasetUploadedRoutingKey, event); err != nil {
		errorMessage := "Failed to publish dataset.uploaded event."
		if updateErr := s.repo.UpdateAnalysisStatusWithError(ctx, job.ID, domain.AnalysisStatusFailed, job.CurrentStep, errorMessage, now); updateErr != nil {
			if s.logger != nil {
				s.logger.Warn("failed to mark analysis job as failed", "jobId", job.ID, "error", updateErr)
			}
		}
		return job.ID, fmt.Errorf("publish event: %w", err)
	}

	if _, supportsClaim := s.repo.(interface {
		ClaimAnalysisStart(context.Context, uuid.UUID, time.Time) (bool, error)
	}); !supportsClaim {
		if err := s.repo.UpdateAnalysisStatus(ctx, job.ID, domain.AnalysisStatusNormalizing, domain.AnalysisStatusNormalizing, now); err != nil {
			return uuid.Nil, fmt.Errorf("update analysis status: %w", err)
		}
	}

	return job.ID, nil
}

func (s *Service) GetAnalysisStatus(ctx context.Context, jobID uuid.UUID) (*AnalysisStatusResponse, error) {
	job, err := s.repo.GetAnalysisJobByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &NotFoundError{Resource: "analysis job", ID: jobID.String()}
		}
		return nil, err
	}
	return buildAnalysisStatusResponse(job), nil
}

func (s *Service) UpdateAnalysisStatus(ctx context.Context, jobID uuid.UUID, status, errorMessage string) (*AnalysisStatusResponse, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status == "" {
		return nil, &InvalidUploadError{Message: "analysis status is required"}
	}
	if !isAllowedStatus(status) {
		return nil, &InvalidUploadError{Message: fmt.Sprintf("unsupported analysis status %q", status)}
	}

	currentStep := status
	if status == domain.AnalysisStatusFailed {
		job, err := s.repo.GetAnalysisJobByID(ctx, jobID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, &NotFoundError{Resource: "analysis job", ID: jobID.String()}
			}
			return nil, err
		}
		currentStep = job.CurrentStep
		if currentStep == "" {
			currentStep = domain.AnalysisStatusFailed
		}
	}

	if err := s.repo.UpdateAnalysisStatusWithError(ctx, jobID, status, currentStep, errorMessage, time.Now().UTC()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &NotFoundError{Resource: "analysis job", ID: jobID.String()}
		}
		return nil, err
	}

	return s.GetAnalysisStatus(ctx, jobID)
}

func parseCSVPreview(reader io.Reader, limit int) (*PreviewResponse, error) {
	csvReader := csv.NewReader(bufio.NewReader(reader))
	csvReader.TrimLeadingSpace = true

	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	headers = trimRecord(headers)
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	rawRows := make([][]string, 0, limit)
	rows := make([]map[string]string, 0, limit)
	truncated := false
	for {
		record, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read csv row: %w", err)
		}
		if len(rawRows) >= limit {
			truncated = true
			break
		}

		record = trimRecord(record)
		rawRows = append(rawRows, record)
		rows = append(rows, recordToPreviewRow(headers, record))
	}

	return &PreviewResponse{
		Headers:   headers,
		Rows:      rows,
		RawRows:   rawRows,
		RowCount:  len(rows),
		Truncated: truncated,
	}, nil
}

func validateUploadedCSV(filename string, data []byte) error {
	if strings.EqualFold(filepath.Ext(filename), ".csv") == false {
		return &InvalidUploadError{Message: "uploaded file must have .csv extension"}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return &InvalidUploadError{Message: "uploaded CSV is empty"}
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return &InvalidUploadError{Message: fmt.Sprintf("failed to read CSV headers: %v", err)}
	}
	headers = trimRecord(headers)
	columns, err := validateHeaders(headers)
	if err != nil {
		return err
	}

	rows := 0
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return &InvalidUploadError{Message: fmt.Sprintf("invalid CSV row: %v", err)}
		}
		record = trimRecord(record)
		rows++

		for _, semanticName := range requiredSemanticColumns {
			index := columns[semanticName]
			if index >= len(record) || strings.TrimSpace(record[index]) == "" {
				return &InvalidUploadError{Message: fmt.Sprintf("row %d has empty required field %s", rows+1, semanticName)}
			}
		}

		if err := validateNumericField(record, columns, "order_amount", rows+1, true); err != nil {
			return err
		}
		if err := validateNumericField(record, columns, "refund_amount", rows+1, false); err != nil {
			return err
		}
		if err := validateNumericField(record, columns, "decision_time_minutes", rows+1, false); err != nil {
			return err
		}
		if err := validateTimestampField(record, columns, rows+1); err != nil {
			return err
		}
	}

	if rows == 0 {
		return &InvalidUploadError{Message: "uploaded CSV must contain at least one data row"}
	}

	return nil
}

func validateHeaders(headers []string) (map[string]int, error) {
	if len(headers) == 0 {
		return nil, invalidIssue("NO_HEADERS", "uploaded CSV has no headers")
	}

	seen := make(map[string]struct{}, len(headers))
	semanticSeen := make(map[string]string)
	columns := make(map[string]int)
	for index, header := range headers {
		normalized := normalizeHeader(header)
		if normalized == "" {
			return nil, invalidIssue("EMPTY_HEADER", "uploaded CSV contains an empty header")
		}
		if _, exists := seen[normalized]; exists {
			return nil, invalidIssue("DUPLICATE_HEADER", fmt.Sprintf("uploaded CSV contains duplicate header %q", header))
		}
		seen[normalized] = struct{}{}

		if semanticName, ok := headerAliases[normalized]; ok {
			if previous, exists := semanticSeen[semanticName]; exists {
				return nil, invalidIssue("DUPLICATE_SEMANTIC_HEADER", fmt.Sprintf("uploaded CSV columns %q and %q map to the same field %s", previous, header, semanticName))
			}
			semanticSeen[semanticName] = header
			columns[semanticName] = index
		}
	}

	for _, required := range requiredSemanticColumns {
		if _, ok := columns[required]; !ok {
			return nil, invalidIssue("MISSING_COLUMN", fmt.Sprintf("uploaded CSV is missing required column %s", required))
		}
	}

	return columns, nil
}

func validateNumericField(record []string, columns map[string]int, semanticName string, rowNumber int, mustBePositive bool) error {
	index, ok := columns[semanticName]
	if !ok {
		return nil
	}
	rawValue := strings.TrimSpace(record[index])
	if rawValue == "" && !mustBePositive {
		return nil
	}
	value, err := parseFlexibleFloat(rawValue)
	if err != nil {
		return &InvalidUploadError{Message: fmt.Sprintf("row %d has invalid numeric value for %s", rowNumber, semanticName)}
	}
	if mustBePositive && value <= 0 {
		return &InvalidUploadError{Message: fmt.Sprintf("row %d must have positive %s", rowNumber, semanticName)}
	}
	if !mustBePositive && value < 0 {
		return &InvalidUploadError{Message: fmt.Sprintf("row %d must not have negative %s", rowNumber, semanticName)}
	}
	return nil
}

func validateTimestampField(record []string, columns map[string]int, rowNumber int) error {
	index, ok := columns["timestamp"]
	if !ok {
		return nil
	}
	value := strings.TrimSpace(record[index])
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02", "02.01.2006 15:04", "01/02/2006 15:04"} {
		if _, err := time.Parse(layout, value); err == nil {
			return nil
		}
	}
	return &InvalidUploadError{Message: fmt.Sprintf("row %d has invalid timestamp", rowNumber)}
}

func parseFlexibleFloat(value string) (float64, error) {
	var cleaned strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case (r >= '0' && r <= '9') || r == '.' || r == ',' || r == '-' || r == '+':
			cleaned.WriteRune(r)
		case unicode.IsSpace(r), r == '$', r == '€', r == '₽', r == '£', r == '\'', r == '_':
			// Accepted formatting/currency characters are removed before parsing.
		default:
			return 0, fmt.Errorf("unsupported character %q in number", r)
		}
	}

	normalized := cleaned.String()
	lastDot := strings.LastIndex(normalized, ".")
	lastComma := strings.LastIndex(normalized, ",")
	switch {
	case lastDot >= 0 && lastComma >= 0 && lastComma > lastDot:
		normalized = strings.ReplaceAll(normalized, ".", "")
		normalized = strings.ReplaceAll(normalized, ",", ".")
	case lastDot >= 0 && lastComma >= 0:
		normalized = strings.ReplaceAll(normalized, ",", "")
	case lastComma >= 0:
		normalized = strings.ReplaceAll(normalized, ",", ".")
	}

	return strconv.ParseFloat(normalized, 64)
}

func trimRecord(record []string) []string {
	trimmed := make([]string, len(record))
	for i, value := range record {
		trimmed[i] = strings.TrimSpace(value)
	}
	return trimmed
}

func recordToPreviewRow(headers, record []string) map[string]string {
	row := make(map[string]string, len(headers))
	for index, header := range headers {
		value := ""
		if index < len(record) {
			value = record[index]
		}
		row[header] = value
	}
	return row
}

func normalizeHeader(value string) string {
	value = strings.TrimPrefix(value, "\ufeff")
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

var requiredSemanticColumns = []string{
	"order_id",
	"customer_id",
	"return_id",
	"support_agent_id",
	"order_amount",
	"refund_amount",
	"decision",
	"timestamp",
}

var headerAliases = map[string]string{
	"order_id":              "order_id",
	"purchase_id":           "order_id",
	"customer_id":           "customer_id",
	"buyer_id":              "customer_id",
	"client_id":             "customer_id",
	"return_id":             "return_id",
	"refund_request_id":     "return_id",
	"support_agent_id":      "support_agent_id",
	"agent_id":              "support_agent_id",
	"support_user_id":       "support_agent_id",
	"order_amount":          "order_amount",
	"purchase_amount":       "order_amount",
	"refund_amount":         "refund_amount",
	"return_amount":         "refund_amount",
	"decision":              "decision",
	"status":                "decision",
	"approval_status":       "decision",
	"timestamp":             "timestamp",
	"created_at":            "timestamp",
	"decision_time":         "timestamp",
	"decision_time_minutes": "decision_time_minutes",
	"resolution_minutes":    "decision_time_minutes",
}

func buildAnalysisStatusResponse(job *domain.AnalysisJob) *AnalysisStatusResponse {
	status := strings.ToUpper(strings.TrimSpace(job.Status))
	currentStep := strings.ToUpper(strings.TrimSpace(job.CurrentStep))
	if currentStep == "" {
		currentStep = status
	}

	message := statusMessage(status)
	progress := statusProgress(status)
	if status == domain.AnalysisStatusFailed && job.Error != "" {
		message = job.Error
	}
	if status == domain.AnalysisStatusFailed {
		progress = statusProgress(currentStep)
	}

	response := &AnalysisStatusResponse{
		ID:              job.ID.String(),
		JobID:           job.ID.String(),
		DatasetID:       job.DatasetID.String(),
		Status:          status,
		CurrentStep:     currentStep,
		Message:         message,
		ProgressPercent: progress,
		Stages:          buildStatusStages(status, currentStep),
		Error:           job.Error,
		FailedStage:     job.FailedStage,
		ResultReady:     job.ResultReady || status == domain.AnalysisStatusCompleted,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
		StartedAt:       job.StartedAt,
		CompletedAt:     job.CompletedAt,
		FailedAt:        job.FailedAt,
	}
	if job.RetryOfJobID != nil {
		response.RetryOfJobID = job.RetryOfJobID.String()
	}
	return response
}

func buildStatusStages(status, currentStep string) []StatusStage {
	stages := make([]StatusStage, 0, len(orderedPipelineStatuses))
	currentIndex := statusIndex(currentStep)
	statusIdx := statusIndex(status)
	if statusIdx > currentIndex {
		currentIndex = statusIdx
	}

	for index, stageStatus := range orderedPipelineStatuses {
		state := "pending"
		switch {
		case status == domain.AnalysisStatusFailed && (stageStatus == currentStep || (currentStep == domain.AnalysisStatusFailed && index == currentIndex)):
			state = "failed"
		case status == domain.AnalysisStatusCompleted || index < currentIndex:
			state = "completed"
		case index == currentIndex:
			state = "current"
		}

		stages = append(stages, StatusStage{
			Status:  stageStatus,
			Message: statusMessage(stageStatus),
			State:   state,
		})
	}

	return stages
}

func statusMessage(status string) string {
	switch status {
	case domain.AnalysisStatusUploaded:
		return "Dataset uploaded and ready to start analysis."
	case domain.AnalysisStatusNormalizing:
		return "Normalizing CSV columns and refund records."
	case domain.AnalysisStatusNormalized:
		return "Dataset normalized and ready for relation building."
	case domain.AnalysisStatusBuildingRelations:
		return "Building customer, order, return and support-agent relations."
	case domain.AnalysisStatusScoring:
		return "Calculating refund approval risk scores."
	case domain.AnalysisStatusCompleted:
		return "Analysis completed. Risk results are ready for review."
	case domain.AnalysisStatusFailed:
		return "Analysis failed. Check the error message for details."
	default:
		return "Analysis status is being updated."
	}
}

func statusProgress(status string) int {
	switch status {
	case domain.AnalysisStatusUploaded:
		return 0
	case domain.AnalysisStatusNormalizing:
		return 20
	case domain.AnalysisStatusNormalized:
		return 40
	case domain.AnalysisStatusBuildingRelations:
		return 60
	case domain.AnalysisStatusScoring:
		return 80
	case domain.AnalysisStatusCompleted:
		return 100
	case domain.AnalysisStatusFailed:
		return 0
	default:
		return 0
	}
}

func statusIndex(status string) int {
	for index, candidate := range orderedPipelineStatuses {
		if candidate == status {
			return index
		}
	}
	return 0
}

func isAllowedStatus(status string) bool {
	for _, candidate := range allowedAnalysisStatuses {
		if candidate == status {
			return true
		}
	}
	return false
}

var orderedPipelineStatuses = []string{
	domain.AnalysisStatusUploaded,
	domain.AnalysisStatusNormalizing,
	domain.AnalysisStatusNormalized,
	domain.AnalysisStatusBuildingRelations,
	domain.AnalysisStatusScoring,
	domain.AnalysisStatusCompleted,
}

var allowedAnalysisStatuses = append([]string{}, append(orderedPipelineStatuses, domain.AnalysisStatusFailed)...)
