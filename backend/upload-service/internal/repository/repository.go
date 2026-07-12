package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"upload-service/internal/domain"
)

var (
	ErrCorrelation     = errors.New("dataset and job correlation mismatch")
	ErrRetryNotAllowed = errors.New("only completed or failed jobs can be retried")
	ErrDatasetArchived = errors.New("dataset is archived")
	ErrArchiveActive   = errors.New("dataset has an active analysis job")
)

const analysisJobColumns = `id, dataset_id, status, current_step, failed_stage,
	error_message, result_ready, retry_of_job_id, created_at, updated_at,
	started_at, completed_at, failed_at`

type DatasetFilter struct {
	Status string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

type DatasetListItem struct {
	Dataset   domain.Dataset
	LatestJob *domain.AnalysisJob
	RowCount  int
	SizeBytes int64
	Warnings  []string
}

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

func (r *Repository) CreateDatasetWithFileAndJob(ctx context.Context, datasetID, jobID uuid.UUID, name, originalFilename, status, filePath, fileType string, sizeBytes int64, rowCount int, warnings []string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `INSERT INTO datasets (id, name, original_filename, status, created_at)
		VALUES ($1, $2, $3, $4, $5)`, datasetID, name, originalFilename, status, now); err != nil {
		return err
	}
	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO uploaded_files
		(id, dataset_id, file_path, file_type, uploaded_at, size_bytes, row_count, warnings)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, uuid.New(), datasetID, filePath, fileType, now, sizeBytes, rowCount, warningsJSON); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO analysis_jobs
		(id, dataset_id, status, current_step, error_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, '', $5, $5)`, jobID, datasetID, status, status, now); err != nil {
		return err
	}
	if err = insertAudit(ctx, tx, datasetID, &jobID, "dataset.uploaded", "", status, "Dataset uploaded and validated.", now); err != nil {
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
	return scanAnalysisJob(r.db.QueryRowContext(ctx, `SELECT `+analysisJobColumns+` FROM analysis_jobs WHERE id = $1`, jobID))
}

func (r *Repository) GetLatestAnalysisJobByDatasetID(ctx context.Context, datasetID uuid.UUID) (*domain.AnalysisJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+analysisJobColumns+`
		 FROM analysis_jobs
		 WHERE dataset_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT 1`,
		datasetID,
	)
	return scanAnalysisJob(row)
}

func (r *Repository) ClaimAnalysisStart(ctx context.Context, jobID uuid.UUID, now time.Time) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var datasetID uuid.UUID
	var status string
	var archivedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT j.dataset_id, j.status, d.archived_at
		FROM analysis_jobs j JOIN datasets d ON d.id = j.dataset_id
		WHERE j.id = $1 FOR UPDATE OF j, d`, jobID).Scan(&datasetID, &status, &archivedAt)
	if err != nil {
		return false, err
	}
	if archivedAt.Valid {
		return false, ErrDatasetArchived
	}
	if status != domain.AnalysisStatusUploaded {
		return false, tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE analysis_jobs SET status=$1, current_step=$1,
		error_message='', failed_stage='', result_ready=FALSE, started_at=COALESCE(started_at,$2),
		updated_at=$2 WHERE id=$3`, domain.AnalysisStatusNormalizing, now, jobID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE datasets SET status=$1 WHERE id=$2`, domain.AnalysisStatusNormalizing, datasetID); err != nil {
		return false, err
	}
	if err = insertAudit(ctx, tx, datasetID, &jobID, "analysis.started", status, domain.AnalysisStatusNormalizing, "Analysis start claimed and dataset.uploaded dispatch requested.", now); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (r *Repository) UpdateAnalysisStatus(ctx context.Context, jobID uuid.UUID, status, currentStep string, updatedAt time.Time) error {
	return r.UpdateAnalysisStatusWithError(ctx, jobID, status, currentStep, "", updatedAt)
}

func (r *Repository) UpdateAnalysisStatusWithError(ctx context.Context, jobID uuid.UUID, status, currentStep, errorMessage string, updatedAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var datasetID uuid.UUID
	var fromStatus string
	if err = tx.QueryRowContext(ctx, `SELECT dataset_id, status FROM analysis_jobs WHERE id=$1 FOR UPDATE`, jobID).Scan(&datasetID, &fromStatus); err != nil {
		return err
	}
	failedStage := ""
	if status == domain.AnalysisStatusFailed {
		failedStage = currentStep
	}
	_, err = tx.ExecContext(ctx, `UPDATE analysis_jobs SET status=$1, current_step=$2,
		error_message=$3, failed_stage=$4, result_ready=$5, updated_at=$6,
		completed_at=CASE WHEN $5 THEN $6 ELSE completed_at END,
		failed_at=CASE WHEN $7 THEN $6 ELSE failed_at END
		WHERE id=$8`, status, currentStep, errorMessage, failedStage, status == domain.AnalysisStatusCompleted, updatedAt, status == domain.AnalysisStatusFailed, jobID)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE datasets SET status=$1 WHERE id=$2`, status, datasetID); err != nil {
		return err
	}
	eventType := "analysis.status.admin"
	if status == domain.AnalysisStatusFailed && strings.Contains(errorMessage, "dataset.uploaded") {
		eventType = "analysis.dispatch.failed"
	}
	if err = insertAudit(ctx, tx, datasetID, &jobID, eventType, fromStatus, status, errorMessage, updatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) TransitionFromEvent(ctx context.Context, jobID, datasetID uuid.UUID, eventType string, allowed []string, toStatus, currentStep, failedStage, errorMessage string, now time.Time) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var actualDatasetID uuid.UUID
	var fromStatus string
	if err = tx.QueryRowContext(ctx, `SELECT dataset_id, status FROM analysis_jobs WHERE id=$1 FOR UPDATE`, jobID).Scan(&actualDatasetID, &fromStatus); err != nil {
		return false, err
	}
	if actualDatasetID != datasetID {
		return false, ErrCorrelation
	}
	if fromStatus == toStatus || fromStatus == domain.AnalysisStatusCompleted || fromStatus == domain.AnalysisStatusFailed {
		return false, tx.Commit()
	}
	transitionAllowed := false
	for _, candidate := range allowed {
		if candidate == fromStatus {
			transitionAllowed = true
			break
		}
	}
	if !transitionAllowed {
		return false, tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `UPDATE analysis_jobs SET status=$1, current_step=$2,
		failed_stage=$3, error_message=$4, result_ready=$5, updated_at=$6,
		completed_at=CASE WHEN $5 THEN $6 ELSE completed_at END,
		failed_at=CASE WHEN $7 THEN $6 ELSE failed_at END
		WHERE id=$8`, toStatus, currentStep, failedStage, errorMessage, toStatus == domain.AnalysisStatusCompleted, now, toStatus == domain.AnalysisStatusFailed, jobID)
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE datasets SET status=$1 WHERE id=$2`, toStatus, datasetID); err != nil {
		return false, err
	}
	if err = insertAudit(ctx, tx, datasetID, &jobID, eventType, fromStatus, toStatus, errorMessage, now); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (r *Repository) ListDatasets(ctx context.Context, filter DatasetFilter) ([]DatasetListItem, int, error) {
	conditions := []string{"d.archived_at IS NULL"}
	args := make([]interface{}, 0, 6)
	if filter.Status != "" {
		args = append(args, strings.ToUpper(filter.Status))
		conditions = append(conditions, fmt.Sprintf("COALESCE(j.status,d.status)=$%d", len(args)))
	}
	if filter.From != nil {
		args = append(args, *filter.From)
		conditions = append(conditions, fmt.Sprintf("d.created_at >= $%d", len(args)))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		conditions = append(conditions, fmt.Sprintf("d.created_at <= $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")
	var total int
	countQuery := `SELECT COUNT(*) FROM datasets d LEFT JOIN LATERAL
		(SELECT status FROM analysis_jobs WHERE dataset_id=d.id ORDER BY created_at DESC,id DESC LIMIT 1) j ON TRUE WHERE ` + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT d.id,d.name,d.original_filename,d.status,d.created_at,d.archived_at,
		COALESCE(f.file_type,'csv'),COALESCE(f.size_bytes,0),COALESCE(f.row_count,0),COALESCE(f.warnings,'[]'::jsonb),
		j.id,j.dataset_id,j.status,j.current_step,j.failed_stage,j.error_message,j.result_ready,j.retry_of_job_id,
		j.created_at,j.updated_at,j.started_at,j.completed_at,j.failed_at
		FROM datasets d
		LEFT JOIN LATERAL (SELECT file_type,size_bytes,row_count,warnings FROM uploaded_files WHERE dataset_id=d.id ORDER BY uploaded_at DESC LIMIT 1) f ON TRUE
		LEFT JOIN LATERAL (SELECT * FROM analysis_jobs WHERE dataset_id=d.id ORDER BY created_at DESC,id DESC LIMIT 1) j ON TRUE
		WHERE ` + where + fmt.Sprintf(" ORDER BY d.created_at DESC,d.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]DatasetListItem, 0)
	for rows.Next() {
		var item DatasetListItem
		var warningsJSON []byte
		var job nullableJob
		if err = rows.Scan(&item.Dataset.ID, &item.Dataset.Name, &item.Dataset.OriginalFilename, &item.Dataset.Status,
			&item.Dataset.UploadedAt, &item.Dataset.ArchivedAt, &item.Dataset.FileType, &item.SizeBytes, &item.RowCount, &warningsJSON,
			&job.ID, &job.DatasetID, &job.Status, &job.CurrentStep, &job.FailedStage, &job.Error, &job.ResultReady,
			&job.RetryOfJobID, &job.CreatedAt, &job.UpdatedAt, &job.StartedAt, &job.CompletedAt, &job.FailedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(warningsJSON, &item.Warnings)
		if job.ID.Valid {
			item.LatestJob = job.value()
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *Repository) GetDataset(ctx context.Context, datasetID uuid.UUID) (*domain.Dataset, error) {
	var dataset domain.Dataset
	var warningsJSON []byte
	err := r.db.QueryRowContext(ctx, `SELECT d.id,d.name,d.original_filename,COALESCE(f.file_type,'csv'),d.status,d.created_at,d.archived_at,
		COALESCE(f.size_bytes,0),COALESCE(f.row_count,0),COALESCE(f.warnings,'[]'::jsonb)
		FROM datasets d LEFT JOIN LATERAL (SELECT file_type,size_bytes,row_count,warnings FROM uploaded_files WHERE dataset_id=d.id ORDER BY uploaded_at DESC LIMIT 1) f ON TRUE
		WHERE d.id=$1`, datasetID).Scan(&dataset.ID, &dataset.Name, &dataset.OriginalFilename, &dataset.FileType, &dataset.Status, &dataset.UploadedAt, &dataset.ArchivedAt, &dataset.SizeBytes, &dataset.RowCount, &warningsJSON)
	if err == nil {
		_ = json.Unmarshal(warningsJSON, &dataset.Warnings)
	}
	return &dataset, err
}

func (r *Repository) ListAnalysisJobs(ctx context.Context, datasetID uuid.UUID) ([]domain.AnalysisJob, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+analysisJobColumns+` FROM analysis_jobs WHERE dataset_id=$1 ORDER BY created_at DESC,id DESC`, datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]domain.AnalysisJob, 0)
	for rows.Next() {
		job, scanErr := scanAnalysisJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

func (r *Repository) ListAuditEvents(ctx context.Context, datasetID uuid.UUID) ([]domain.AuditEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,dataset_id,job_id,event_type,from_status,to_status,message,created_at
		FROM lifecycle_audit_events WHERE dataset_id=$1 ORDER BY created_at DESC,id DESC`, datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var event domain.AuditEvent
		if err = rows.Scan(&event.ID, &event.DatasetID, &event.JobID, &event.EventType, &event.FromStatus, &event.ToStatus, &event.Message, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *Repository) CreateRetryJob(ctx context.Context, sourceJobID, newJobID uuid.UUID, now time.Time) (*domain.AnalysisJob, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	var datasetID uuid.UUID
	var sourceStatus string
	var archivedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT j.dataset_id,j.status,d.archived_at FROM analysis_jobs j
		JOIN datasets d ON d.id=j.dataset_id WHERE j.id=$1 FOR UPDATE OF j,d`, sourceJobID).Scan(&datasetID, &sourceStatus, &archivedAt)
	if err != nil {
		return nil, false, err
	}
	if archivedAt.Valid {
		return nil, false, ErrDatasetArchived
	}
	if sourceStatus != domain.AnalysisStatusCompleted && sourceStatus != domain.AnalysisStatusFailed {
		return nil, false, ErrRetryNotAllowed
	}
	existing, existingErr := scanAnalysisJob(tx.QueryRowContext(ctx, `SELECT `+analysisJobColumns+` FROM analysis_jobs WHERE retry_of_job_id=$1`, sourceJobID))
	if existingErr == nil {
		if err = tx.Commit(); err != nil {
			return nil, false, err
		}
		return existing, false, nil
	}
	if !errors.Is(existingErr, sql.ErrNoRows) {
		return nil, false, existingErr
	}
	var activeJobs int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM analysis_jobs
		WHERE dataset_id=$1 AND status NOT IN ($2,$3)`, datasetID, domain.AnalysisStatusCompleted, domain.AnalysisStatusFailed).Scan(&activeJobs); err != nil {
		return nil, false, err
	}
	if activeJobs > 0 {
		return nil, false, ErrRetryNotAllowed
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO analysis_jobs
		(id,dataset_id,status,current_step,error_message,retry_of_job_id,created_at,updated_at)
		VALUES ($1,$2,$3,$3,'',$4,$5,$5)`, newJobID, datasetID, domain.AnalysisStatusUploaded, sourceJobID, now)
	if err != nil {
		return nil, false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE datasets SET status=$1 WHERE id=$2`, domain.AnalysisStatusUploaded, datasetID); err != nil {
		return nil, false, err
	}
	message := fmt.Sprintf("Retry created from job %s.", sourceJobID)
	if err = insertAudit(ctx, tx, datasetID, &newJobID, "analysis.retry.created", sourceStatus, domain.AnalysisStatusUploaded, message, now); err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	job, err := r.GetAnalysisJobByID(ctx, newJobID)
	return job, true, err
}

func (r *Repository) ArchiveDataset(ctx context.Context, datasetID uuid.UUID, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var archivedAt sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT archived_at FROM datasets WHERE id=$1 FOR UPDATE`, datasetID).Scan(&archivedAt); err != nil {
		return err
	}
	if archivedAt.Valid {
		return tx.Commit()
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM analysis_jobs WHERE dataset_id=$1 AND status NOT IN ($2,$3)`, datasetID, domain.AnalysisStatusCompleted, domain.AnalysisStatusFailed).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return ErrArchiveActive
	}
	if _, err = tx.ExecContext(ctx, `UPDATE datasets SET archived_at=$1 WHERE id=$2`, now, datasetID); err != nil {
		return err
	}
	if err = insertAudit(ctx, tx, datasetID, nil, "dataset.archived", "", "ARCHIVED", "Dataset archived; artifacts and analysis history retained.", now); err != nil {
		return err
	}
	return tx.Commit()
}

type rowScanner interface{ Scan(...interface{}) error }

func scanAnalysisJob(row rowScanner) (*domain.AnalysisJob, error) {
	var job domain.AnalysisJob
	err := row.Scan(&job.ID, &job.DatasetID, &job.Status, &job.CurrentStep, &job.FailedStage,
		&job.Error, &job.ResultReady, &job.RetryOfJobID, &job.CreatedAt, &job.UpdatedAt,
		&job.StartedAt, &job.CompletedAt, &job.FailedAt)
	return &job, err
}

type nullableJob struct {
	ID, DatasetID, Status, CurrentStep, FailedStage, Error, RetryOfJobID sql.NullString
	ResultReady                                                          sql.NullBool
	CreatedAt, UpdatedAt, StartedAt, CompletedAt, FailedAt               sql.NullTime
}

func (j nullableJob) value() *domain.AnalysisJob {
	job := &domain.AnalysisJob{Status: j.Status.String, CurrentStep: j.CurrentStep.String, FailedStage: j.FailedStage.String,
		Error: j.Error.String, ResultReady: j.ResultReady.Bool, CreatedAt: j.CreatedAt.Time, UpdatedAt: j.UpdatedAt.Time}
	job.ID, _ = uuid.Parse(j.ID.String)
	job.DatasetID, _ = uuid.Parse(j.DatasetID.String)
	if j.RetryOfJobID.Valid {
		id, _ := uuid.Parse(j.RetryOfJobID.String)
		job.RetryOfJobID = &id
	}
	if j.StartedAt.Valid {
		value := j.StartedAt.Time
		job.StartedAt = &value
	}
	if j.CompletedAt.Valid {
		value := j.CompletedAt.Time
		job.CompletedAt = &value
	}
	if j.FailedAt.Valid {
		value := j.FailedAt.Time
		job.FailedAt = &value
	}
	return job
}

func insertAudit(ctx context.Context, tx *sql.Tx, datasetID uuid.UUID, jobID *uuid.UUID, eventType, fromStatus, toStatus, message string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO lifecycle_audit_events
		(dataset_id,job_id,event_type,from_status,to_status,message,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		datasetID, jobID, eventType, fromStatus, toStatus, message, now)
	return err
}
