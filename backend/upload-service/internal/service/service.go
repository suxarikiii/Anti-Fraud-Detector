package service

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"time"

	"upload-service/internal/domain"
	"upload-service/internal/repository"
	"upload-service/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type PreviewResponse struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

type Service struct {
	repo      *repository.Repository
	minio     *minio.Client
	bucket    string
	publisher *rabbitmq.Publisher
	logger    *slog.Logger
}

type datasetUploadedEvent struct {
	DatasetID  string `json:"datasetId"`
	JobID      string `json:"jobId"`
	FilePath   string `json:"filePath"`
	FileType   string `json:"fileType"`
	UploadedAt string `json:"uploadedAt"`
}

func NewService(repo *repository.Repository, minioClient *minio.Client, bucket string, publisher *rabbitmq.Publisher, logger *slog.Logger) *Service {
	return &Service{repo: repo, minio: minioClient, bucket: bucket, publisher: publisher, logger: logger}
}

func (s *Service) UploadDataset(ctx context.Context, file io.Reader, size int64, originalFilename string) (uuid.UUID, uuid.UUID, error) {
	datasetID := uuid.New()
	objectName := fmt.Sprintf("datasets/%s.csv", datasetID.String())
	if _, err := s.minio.PutObject(ctx, s.bucket, objectName, file, size, minio.PutObjectOptions{ContentType: "text/csv"}); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("upload to minio: %w", err)
	}

	now := time.Now().UTC()
	if err := s.repo.CreateDatasetWithFile(ctx, datasetID, originalFilename, originalFilename, "UPLOADED", objectName, "csv", now, now); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("create dataset records: %w", err)
	}

	// create analysis job immediately after upload
	jobID := uuid.New()
	if err := s.repo.CreateAnalysisJob(ctx, jobID, datasetID, "UPLOADED", "UPLOADED", now, now); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("create analysis job: %w", err)
	}

	// publish dataset.uploaded event
	event := datasetUploadedEvent{
		DatasetID:  datasetID.String(),
		JobID:      jobID.String(),
		FilePath:   objectName,
		FileType:   "csv",
		UploadedAt: now.UTC().Format(time.RFC3339),
	}

	if err := s.publisher.Publish(ctx, "dataset.uploaded", event); err != nil {
		s.logger.Error("failed to publish dataset.uploaded", "error", err)
	}

	return datasetID, jobID, nil
}

func (s *Service) PreviewDataset(ctx context.Context, datasetID uuid.UUID) (*PreviewResponse, error) {
	uploadedFile, err := s.repo.GetUploadedFileByDatasetID(ctx, datasetID)
	if err != nil {
		return nil, err
	}

	object, err := s.minio.GetObject(ctx, s.bucket, uploadedFile.FilePath, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio get object: %w", err)
	}
	defer object.Close()

	scanner := csv.NewReader(bufio.NewReader(object))
	rows := make([][]string, 0, 20)

	headers, err := scanner.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	for len(rows) < 20 {
		record, err := scanner.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read csv row: %w", err)
		}
		rows = append(rows, record)
	}

	return &PreviewResponse{Headers: headers, Rows: rows}, nil
}

func (s *Service) StartAnalysis(ctx context.Context, datasetID uuid.UUID) (uuid.UUID, error) {
	uploadedFile, err := s.repo.GetUploadedFileByDatasetID(ctx, datasetID)
	if err != nil {
		return uuid.Nil, err
	}

	jobID := uuid.New()
	now := time.Now().UTC()
	if err := s.repo.CreateAnalysisJob(ctx, jobID, datasetID, "UPLOADED", "UPLOADED", now, now); err != nil {
		return uuid.Nil, fmt.Errorf("create analysis job: %w", err)
	}

	event := datasetUploadedEvent{
		DatasetID:  datasetID.String(),
		JobID:      jobID.String(),
		FilePath:   uploadedFile.FilePath,
		FileType:   uploadedFile.FileType,
		UploadedAt: uploadedFile.UploadedAt.UTC().Format(time.RFC3339),
	}

	if err := s.publisher.Publish(ctx, "dataset.uploaded", event); err != nil {
		return uuid.Nil, fmt.Errorf("publish event: %w", err)
	}

	return jobID, nil
}

func (s *Service) GetAnalysisStatus(ctx context.Context, jobID uuid.UUID) (*domain.AnalysisJob, error) {
	job, err := s.repo.GetAnalysisJobByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return job, nil
}
