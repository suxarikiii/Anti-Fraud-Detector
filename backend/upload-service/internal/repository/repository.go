package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"upload-service/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateDatasetWithFile(ctx context.Context, datasetID uuid.UUID, name, originalFilename, status, filePath, fileType string, uploadedAt, createdAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO datasets (id, name, original_filename, status, created_at) VALUES ($1, $2, $3, $4, $5)`,
		datasetID,
		name,
		originalFilename,
		status,
		createdAt,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO uploaded_files (id, dataset_id, file_path, file_type, uploaded_at) VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(),
		datasetID,
		filePath,
		fileType,
		uploadedAt,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) GetUploadedFileByDatasetID(ctx context.Context, datasetID uuid.UUID) (*domain.UploadedFile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, dataset_id, file_path, file_type, uploaded_at FROM uploaded_files WHERE dataset_id = $1 ORDER BY uploaded_at DESC LIMIT 1`,
		datasetID,
	)

	var file domain.UploadedFile
	if err := row.Scan(&file.ID, &file.DatasetID, &file.FilePath, &file.FileType, &file.UploadedAt); err != nil {
		return nil, err
	}

	return &file, nil
}

func (r *Repository) CreateAnalysisJob(ctx context.Context, jobID, datasetID uuid.UUID, status, currentStep string, createdAt, updatedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO analysis_jobs (id, dataset_id, status, current_step, error_message, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		jobID,
		datasetID,
		status,
		currentStep,
		"",
		createdAt,
		updatedAt,
	)
	return err
}

func (r *Repository) GetAnalysisJobByID(ctx context.Context, jobID uuid.UUID) (*domain.AnalysisJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, dataset_id, status, current_step, error_message, created_at, updated_at FROM analysis_jobs WHERE id = $1`,
		jobID,
	)

	var job domain.AnalysisJob
	if err := row.Scan(&job.ID, &job.DatasetID, &job.Status, &job.CurrentStep, &job.Error, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *Repository) GetLatestAnalysisJobByDatasetID(ctx context.Context, datasetID uuid.UUID) (*domain.AnalysisJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, dataset_id, status, current_step, error_message, created_at, updated_at
		 FROM analysis_jobs
		 WHERE dataset_id = $1
		 ORDER BY created_at DESC
		 LIMIT 1`,
		datasetID,
	)

	var job domain.AnalysisJob
	if err := row.Scan(&job.ID, &job.DatasetID, &job.Status, &job.CurrentStep, &job.Error, &job.CreatedAt, &job.UpdatedAt); err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *Repository) UpdateAnalysisStatus(ctx context.Context, jobID uuid.UUID, status, currentStep string, updatedAt time.Time) error {
	return r.UpdateAnalysisStatusWithError(ctx, jobID, status, currentStep, "", updatedAt)
}

func (r *Repository) UpdateAnalysisStatusWithError(ctx context.Context, jobID uuid.UUID, status, currentStep, errorMessage string, updatedAt time.Time) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE analysis_jobs SET status = $1, current_step = $2, error_message = $3, updated_at = $4 WHERE id = $5`,
		status,
		currentStep,
		errorMessage,
		updatedAt,
		jobID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
