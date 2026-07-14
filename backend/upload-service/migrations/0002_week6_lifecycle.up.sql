ALTER TABLE datasets
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMP WITH TIME ZONE;

ALTER TABLE uploaded_files
    ADD COLUMN IF NOT EXISTS size_bytes BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS row_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS warnings JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE analysis_jobs
    ADD COLUMN IF NOT EXISTS failed_stage VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS result_ready BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS retry_of_job_id UUID REFERENCES analysis_jobs(id),
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS failed_at TIMESTAMP WITH TIME ZONE;

UPDATE analysis_jobs
SET result_ready = TRUE,
    completed_at = COALESCE(completed_at, updated_at),
    started_at = COALESCE(started_at, created_at)
WHERE status = 'COMPLETED';

UPDATE analysis_jobs
SET failed_stage = CASE WHEN failed_stage = '' THEN current_step ELSE failed_stage END,
    failed_at = COALESCE(failed_at, updated_at),
    started_at = COALESCE(started_at, created_at)
WHERE status = 'FAILED';

UPDATE analysis_jobs
SET started_at = COALESCE(started_at, created_at)
WHERE status NOT IN ('UPLOADED', 'COMPLETED', 'FAILED');

CREATE UNIQUE INDEX IF NOT EXISTS uq_analysis_jobs_retry_of
    ON analysis_jobs(retry_of_job_id) WHERE retry_of_job_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_analysis_jobs_dataset_created
    ON analysis_jobs(dataset_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analysis_jobs_status_updated
    ON analysis_jobs(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_datasets_created
    ON datasets(created_at DESC);

CREATE TABLE IF NOT EXISTS lifecycle_audit_events (
    id BIGSERIAL PRIMARY KEY,
    dataset_id UUID NOT NULL REFERENCES datasets(id),
    job_id UUID REFERENCES analysis_jobs(id),
    event_type VARCHAR(100) NOT NULL,
    from_status VARCHAR(50) NOT NULL DEFAULT '',
    to_status VARCHAR(50) NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_dataset_created
    ON lifecycle_audit_events(dataset_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_job_created
    ON lifecycle_audit_events(job_id, created_at ASC);
