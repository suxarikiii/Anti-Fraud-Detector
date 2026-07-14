DROP TABLE IF EXISTS lifecycle_audit_events;
DROP INDEX IF EXISTS idx_datasets_created;
DROP INDEX IF EXISTS idx_analysis_jobs_status_updated;
DROP INDEX IF EXISTS idx_analysis_jobs_dataset_created;
DROP INDEX IF EXISTS uq_analysis_jobs_retry_of;
ALTER TABLE analysis_jobs
    DROP COLUMN IF EXISTS failed_at,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS started_at,
    DROP COLUMN IF EXISTS retry_of_job_id,
    DROP COLUMN IF EXISTS result_ready,
    DROP COLUMN IF EXISTS failed_stage;
ALTER TABLE uploaded_files
    DROP COLUMN IF EXISTS warnings,
    DROP COLUMN IF EXISTS row_count,
    DROP COLUMN IF EXISTS size_bytes;
ALTER TABLE datasets DROP COLUMN IF EXISTS archived_at;
