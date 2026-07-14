package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"upload-service/internal/domain"
	"upload-service/internal/repository"
)

func TestValidationAggregatesPolicyErrors(t *testing.T) {
	body := `order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,decision,timestamp
order_1,customer_1,return_1,agent_1,-5,-1,maybe,not-a-time
order_2,customer_2,return_1,agent_2,10,0,APPROVED,2026-06-01T09:06:00Z
`
	_, err := validateCSVStream(strings.NewReader(body), 100, 100)
	var invalid *InvalidUploadError
	if !errors.As(err, &invalid) {
		t.Fatalf("error=%T, want InvalidUploadError", err)
	}
	if len(invalid.Errors) < 5 {
		t.Fatalf("validation errors=%d, want aggregated errors", len(invalid.Errors))
	}
	codes := map[string]bool{}
	for _, issue := range invalid.Errors {
		codes[issue.Code] = true
	}
	for _, code := range []string{"NON_POSITIVE_AMOUNT", "NEGATIVE_VALUE", "INVALID_DECISION", "INVALID_TIMESTAMP", "DUPLICATE_RETURN_ID"} {
		if !codes[code] {
			t.Errorf("missing validation code %s", code)
		}
	}
	if len(invalid.Warnings) != 1 || invalid.Warnings[0].Code != "ZERO_REFUND_AMOUNT" {
		t.Fatalf("warnings=%v, want zero refund warning", invalid.Warnings)
	}
}

func TestValidationEnforcesRowLimit(t *testing.T) {
	_, err := validateCSVStream(strings.NewReader(cleanRefundCSV), 1, 10)
	var invalid *InvalidUploadError
	if !errors.As(err, &invalid) {
		t.Fatalf("error=%v, want validation error", err)
	}
	if invalid.Errors[len(invalid.Errors)-1].Code != "TOO_MANY_ROWS" {
		t.Fatalf("errors=%v", invalid.Errors)
	}
}

func TestUploadRejectsOversizeBeforeObjectStore(t *testing.T) {
	svc := NewServiceWithStore(newFakeRepo(), newFakeStore(), &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.ConfigureUploadLimits(8, 100, 10)
	_, err := svc.UploadDatasetDetailed(context.Background(), strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv", "text/csv")
	var invalid *InvalidUploadError
	if !errors.As(err, &invalid) || invalid.Errors[0].Code != "FILE_TOO_LARGE" {
		t.Fatalf("error=%v, want FILE_TOO_LARGE", err)
	}
}

func TestFilenameAndNumericValidationHardening(t *testing.T) {
	if got := sanitizeFilename(`../../unsafe\name.csv`); got != "name.csv" {
		t.Fatalf("sanitized filename=%q", got)
	}
	body := strings.Replace(cleanRefundCSV, "203.84", "amount203.84", 1)
	_, err := validateCSVStream(strings.NewReader(body), 100, 10)
	var invalid *InvalidUploadError
	if !errors.As(err, &invalid) || invalid.Errors[0].Code != "INVALID_NUMBER" {
		t.Fatalf("error=%v, want INVALID_NUMBER", err)
	}
}

func TestUploadHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := NewServiceWithStore(newFakeRepo(), newFakeStore(), &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := svc.UploadDatasetDetailed(ctx, strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv", "text/csv")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want context cancellation", err)
	}
}

type atomicFailRepo struct{ *fakeRepo }

func (r *atomicFailRepo) CreateDatasetWithFileAndJob(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, string, int64, int, []string, time.Time) error {
	return errors.New("postgres unavailable")
}

type deleteTrackingStore struct {
	*fakeStore
	deleted bool
}

type failingPutStore struct{}

func (failingPutStore) Put(context.Context, string, io.Reader, int64, string) error {
	return errors.New("minio unavailable")
}
func (failingPutStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("minio unavailable")
}

func TestUploadStopsBeforeDatabaseWhenObjectStoreFails(t *testing.T) {
	repo := newFakeRepo()
	svc := NewServiceWithStore(repo, failingPutStore{}, &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := svc.UploadDatasetDetailed(context.Background(), strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv", "text/csv")
	if err == nil || !strings.Contains(err.Error(), "upload object") {
		t.Fatalf("error=%v, want object store error", err)
	}
	if len(repo.files) != 0 || len(repo.jobs) != 0 {
		t.Fatalf("database changed after object failure: files=%d jobs=%d", len(repo.files), len(repo.jobs))
	}
}

func (s *deleteTrackingStore) Delete(_ context.Context, objectName string) error {
	s.deleted = true
	delete(s.objects, objectName)
	return nil
}

func TestUploadCompensatesObjectWhenDatabaseTransactionFails(t *testing.T) {
	repo := &atomicFailRepo{fakeRepo: newFakeRepo()}
	store := &deleteTrackingStore{fakeStore: newFakeStore()}
	svc := NewServiceWithStore(repo, store, &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := svc.UploadDatasetDetailed(context.Background(), strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv", "text/csv")
	if err == nil || !strings.Contains(err.Error(), "create upload records") {
		t.Fatalf("error=%v, want database transaction error", err)
	}
	if !store.deleted || len(store.objects) != 0 {
		t.Fatalf("compensation deleted=%v objects=%d", store.deleted, len(store.objects))
	}
}

func TestPipelineEventsCompleteAndAreIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := NewServiceWithStore(repo, newFakeStore(), &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	datasetID, jobID, err := svc.UploadDataset(ctx, strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv")
	if err != nil {
		t.Fatal(err)
	}
	repo.jobs[jobID].Status = domain.AnalysisStatusNormalizing
	repo.jobs[jobID].CurrentStep = domain.AnalysisStatusNormalizing
	event := func(key string, extra map[string]string) error {
		payload := map[string]string{"datasetId": datasetID.String(), "jobId": jobID.String()}
		for k, v := range extra {
			payload[k] = v
		}
		body, _ := json.Marshal(payload)
		return svc.HandlePipelineEvent(ctx, key, body)
	}
	for _, key := range []string{DatasetNormalizedRoutingKey, RelationsBuiltRoutingKey, ScoringCompletedRoutingKey} {
		if err := event(key, nil); err != nil {
			t.Fatalf("handle %s: %v", key, err)
		}
		if err := event(key, nil); err != nil {
			t.Fatalf("duplicate %s: %v", key, err)
		}
	}
	status, err := svc.GetAnalysisStatus(ctx, jobID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != domain.AnalysisStatusCompleted || !status.ResultReady || status.ProgressPercent != 100 {
		t.Fatalf("status=%+v", status)
	}
}

func TestPipelineEventRejectsWrongCorrelationAndIgnoresOutOfOrder(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := NewServiceWithStore(repo, newFakeStore(), &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	datasetID, jobID, err := svc.UploadDataset(ctx, strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(PipelineEvent{DatasetID: uuid.NewString(), JobID: jobID.String()})
	err = svc.HandlePipelineEvent(ctx, DatasetNormalizedRoutingKey, body)
	var permanent *PermanentEventError
	if !errors.As(err, &permanent) {
		t.Fatalf("error=%v, want permanent correlation error", err)
	}
	body, _ = json.Marshal(PipelineEvent{DatasetID: datasetID.String(), JobID: jobID.String()})
	if err = svc.HandlePipelineEvent(ctx, ScoringCompletedRoutingKey, body); err != nil {
		t.Fatal(err)
	}
	if repo.jobs[jobID].Status != domain.AnalysisStatusUploaded {
		t.Fatalf("out-of-order event changed status to %s", repo.jobs[jobID].Status)
	}
}

func TestPipelineFailureRecordsStage(t *testing.T) {
	tests := []struct{ stage, status string }{
		{stage: "NORMALIZATION", status: domain.AnalysisStatusNormalizing},
		{stage: "RELATIONS", status: domain.AnalysisStatusBuildingRelations},
		{stage: "SCORING", status: domain.AnalysisStatusScoring},
	}
	for _, test := range tests {
		t.Run(test.stage, func(t *testing.T) {
			ctx := context.Background()
			repo := newFakeRepo()
			svc := NewServiceWithStore(repo, newFakeStore(), &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			datasetID, jobID, err := svc.UploadDataset(ctx, strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv")
			if err != nil {
				t.Fatal(err)
			}
			repo.jobs[jobID].Status = test.status
			repo.jobs[jobID].CurrentStep = test.status
			body, _ := json.Marshal(PipelineEvent{DatasetID: datasetID.String(), JobID: jobID.String(), FailedStep: test.stage, ErrorMessage: "downstream failed"})
			if err = svc.HandlePipelineEvent(ctx, PipelineFailedRoutingKey, body); err != nil {
				t.Fatal(err)
			}
			status, _ := svc.GetAnalysisStatus(ctx, jobID)
			if status.Status != domain.AnalysisStatusFailed || status.FailedStage == "" || status.Error != "downstream failed" {
				t.Fatalf("status=%+v", status)
			}
		})
	}
}

func TestPipelineFailureAcceptsRelationsEventAliases(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := NewServiceWithStore(repo, newFakeStore(), &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	datasetID, jobID, err := svc.UploadDataset(ctx, strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv")
	if err != nil {
		t.Fatal(err)
	}
	repo.jobs[jobID].Status = domain.AnalysisStatusBuildingRelations
	body, _ := json.Marshal(map[string]string{"datasetId": datasetID.String(), "jobId": jobID.String(), "stage": "RELATIONS", "message": "relations unavailable"})
	if err = svc.HandlePipelineEvent(ctx, PipelineFailedRoutingKey, body); err != nil {
		t.Fatal(err)
	}
	status, _ := svc.GetAnalysisStatus(ctx, jobID)
	if status.Status != domain.AnalysisStatusFailed || status.FailedStage != domain.AnalysisStatusBuildingRelations || status.Error != "relations unavailable" {
		t.Fatalf("status=%+v", status)
	}
}

func TestOutOfOrderFailureFromCompletedStageIsIgnored(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := NewServiceWithStore(repo, newFakeStore(), &fakePublisher{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	datasetID, jobID, err := svc.UploadDataset(ctx, strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv")
	if err != nil {
		t.Fatal(err)
	}
	repo.jobs[jobID].Status = domain.AnalysisStatusScoring
	repo.jobs[jobID].CurrentStep = domain.AnalysisStatusScoring
	body, _ := json.Marshal(PipelineEvent{DatasetID: datasetID.String(), JobID: jobID.String(), FailedStep: "NORMALIZATION", ErrorMessage: "late failure"})
	if err = svc.HandlePipelineEvent(ctx, PipelineFailedRoutingKey, body); err != nil {
		t.Fatal(err)
	}
	if repo.jobs[jobID].Status != domain.AnalysisStatusScoring {
		t.Fatalf("late failure changed status to %s", repo.jobs[jobID].Status)
	}
}

type claimingRepo struct {
	*fakeRepo
	mu sync.Mutex
}

func (r *claimingRepo) GetLatestAnalysisJobByDatasetID(_ context.Context, datasetID uuid.UUID) (*domain.AnalysisJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest *domain.AnalysisJob
	for _, job := range r.jobs {
		if job.DatasetID == datasetID && (latest == nil || job.CreatedAt.After(latest.CreatedAt)) {
			copy := *job
			latest = &copy
		}
	}
	if latest == nil {
		return nil, errors.New("sql: no rows in result set")
	}
	return latest, nil
}

func (r *claimingRepo) ClaimAnalysisStart(_ context.Context, jobID uuid.UUID, now time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[jobID]
	if job == nil {
		return false, errors.New("missing")
	}
	if job.Status != domain.AnalysisStatusUploaded {
		return false, nil
	}
	job.Status = domain.AnalysisStatusNormalizing
	job.CurrentStep = domain.AnalysisStatusNormalizing
	job.UpdatedAt = now
	return true, nil
}

func TestConcurrentStartPublishesOnce(t *testing.T) {
	ctx := context.Background()
	base := newFakeRepo()
	repo := &claimingRepo{fakeRepo: base}
	publisher := &lockedPublisher{}
	svc := NewServiceWithStore(repo, newFakeStore(), publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
	datasetID, _, err := svc.UploadDataset(ctx, strings.NewReader(cleanRefundCSV), int64(len(cleanRefundCSV)), "refunds.csv")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = svc.StartAnalysis(ctx, datasetID) }()
	}
	wg.Wait()
	if publisher.count != 1 {
		t.Fatalf("published=%d, want 1", publisher.count)
	}
}

type lockedPublisher struct {
	mu    sync.Mutex
	count int
}

func (p *lockedPublisher) Publish(context.Context, string, interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.count++
	return nil
}

func (r *fakeRepo) TransitionFromEvent(_ context.Context, jobID, datasetID uuid.UUID, _ string, allowed []string, toStatus, currentStep, failedStage, errorMessage string, now time.Time) (bool, error) {
	job, ok := r.jobs[jobID]
	if !ok {
		return false, errors.New("sql: no rows in result set")
	}
	if job.DatasetID != datasetID {
		return false, repository.ErrCorrelation
	}
	if job.Status == toStatus || job.Status == domain.AnalysisStatusCompleted || job.Status == domain.AnalysisStatusFailed {
		return false, nil
	}
	valid := false
	for _, status := range allowed {
		if status == job.Status {
			valid = true
		}
	}
	if !valid {
		return false, nil
	}
	job.Status = toStatus
	job.CurrentStep = currentStep
	job.FailedStage = failedStage
	job.Error = errorMessage
	job.ResultReady = toStatus == domain.AnalysisStatusCompleted
	job.UpdatedAt = now
	if toStatus == domain.AnalysisStatusCompleted {
		job.CompletedAt = &now
	}
	if toStatus == domain.AnalysisStatusFailed {
		job.FailedAt = &now
	}
	return true, nil
}
