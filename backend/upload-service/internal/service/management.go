package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"upload-service/internal/domain"
	"upload-service/internal/repository"
)

type DatasetListResponse struct {
	Items      []DatasetSummary `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	Total      int              `json:"total"`
	TotalPages int              `json:"totalPages"`
}
type DatasetSummary struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Filename     string    `json:"filename"`
	Status       string    `json:"status"`
	ResultReady  bool      `json:"resultReady"`
	LatestJobID  string    `json:"latestJobId,omitempty"`
	RowCount     int       `json:"rowCount"`
	SizeBytes    int64     `json:"sizeBytes"`
	WarningCount int       `json:"warningCount"`
	UploadedAt   time.Time `json:"uploadedAt"`
}
type DatasetDetailsResponse struct {
	Dataset domain.Dataset            `json:"dataset"`
	Jobs    []*AnalysisStatusResponse `json:"analysisHistory"`
	Audit   []domain.AuditEvent       `json:"auditEvents"`
}

type datasetListRepository interface {
	ListDatasets(context.Context, repository.DatasetFilter) ([]repository.DatasetListItem, int, error)
}
type datasetDetailsRepository interface {
	GetDataset(context.Context, uuid.UUID) (*domain.Dataset, error)
	ListAnalysisJobs(context.Context, uuid.UUID) ([]domain.AnalysisJob, error)
	ListAuditEvents(context.Context, uuid.UUID) ([]domain.AuditEvent, error)
}
type archiveRepository interface {
	ArchiveDataset(context.Context, uuid.UUID, time.Time) error
}
type retryRepository interface {
	CreateRetryJob(context.Context, uuid.UUID, uuid.UUID, time.Time) (*domain.AnalysisJob, bool, error)
}

func (s *Service) ListDatasets(ctx context.Context, status string, from, to *time.Time, page, pageSize int) (*DatasetListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "" && !isAllowedStatus(status) {
		return nil, &InvalidUploadError{Message: fmt.Sprintf("unsupported status filter %q", status)}
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, &InvalidUploadError{Message: "from must not be later than to"}
	}
	repo, ok := s.repo.(datasetListRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support dataset management")
	}
	items, total, err := repo.ListDatasets(ctx, repository.DatasetFilter{Status: status, From: from, To: to, Limit: pageSize, Offset: (page - 1) * pageSize})
	if err != nil {
		return nil, err
	}
	result := make([]DatasetSummary, 0, len(items))
	for _, item := range items {
		summary := DatasetSummary{ID: item.Dataset.ID.String(), Name: item.Dataset.Name, Filename: item.Dataset.OriginalFilename, Status: item.Dataset.Status, RowCount: item.RowCount, SizeBytes: item.SizeBytes, WarningCount: len(item.Warnings), UploadedAt: item.Dataset.UploadedAt}
		if item.LatestJob != nil {
			summary.Status = item.LatestJob.Status
			summary.ResultReady = item.LatestJob.ResultReady || item.LatestJob.Status == domain.AnalysisStatusCompleted
			summary.LatestJobID = item.LatestJob.ID.String()
		}
		result = append(result, summary)
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return &DatasetListResponse{Items: result, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}, nil
}

func (s *Service) GetDatasetDetails(ctx context.Context, datasetID uuid.UUID) (*DatasetDetailsResponse, error) {
	repo, ok := s.repo.(datasetDetailsRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support dataset management")
	}
	dataset, err := repo.GetDataset(ctx, datasetID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &NotFoundError{Resource: "dataset", ID: datasetID.String()}
	}
	if err != nil {
		return nil, err
	}
	jobs, err := repo.ListAnalysisJobs(ctx, datasetID)
	if err != nil {
		return nil, err
	}
	audit, err := repo.ListAuditEvents(ctx, datasetID)
	if err != nil {
		return nil, err
	}
	responses := make([]*AnalysisStatusResponse, 0, len(jobs))
	for i := range jobs {
		responses = append(responses, buildAnalysisStatusResponse(&jobs[i]))
	}
	return &DatasetDetailsResponse{Dataset: *dataset, Jobs: responses, Audit: audit}, nil
}

func (s *Service) RetryAnalysis(ctx context.Context, sourceJobID uuid.UUID) (uuid.UUID, error) {
	repo, ok := s.repo.(retryRepository)
	if !ok {
		return uuid.Nil, fmt.Errorf("repository does not support retry")
	}
	job, _, err := repo.CreateRetryJob(ctx, sourceJobID, uuid.New(), time.Now().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, &NotFoundError{Resource: "analysis job", ID: sourceJobID.String()}
	}
	if errors.Is(err, repository.ErrRetryNotAllowed) {
		return uuid.Nil, &InvalidUploadError{Message: err.Error()}
	}
	if errors.Is(err, repository.ErrDatasetArchived) {
		return uuid.Nil, &InvalidUploadError{Message: err.Error()}
	}
	if err != nil {
		return uuid.Nil, err
	}
	uploadedFile, err := s.repo.GetUploadedFileByDatasetID(ctx, job.DatasetID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, &NotFoundError{Resource: "dataset artifact", ID: job.DatasetID.String()}
	}
	if err != nil {
		return uuid.Nil, err
	}
	return s.startAnalysisJob(ctx, uploadedFile, job, time.Now().UTC())
}

func (s *Service) ArchiveDataset(ctx context.Context, datasetID uuid.UUID) error {
	repo, ok := s.repo.(archiveRepository)
	if !ok {
		return fmt.Errorf("repository does not support archive")
	}
	err := repo.ArchiveDataset(ctx, datasetID, time.Now().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return &NotFoundError{Resource: "dataset", ID: datasetID.String()}
	}
	if errors.Is(err, repository.ErrArchiveActive) {
		return &InvalidUploadError{Message: err.Error()}
	}
	return err
}
