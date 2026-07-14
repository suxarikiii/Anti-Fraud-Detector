package service

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
	"upload-service/internal/repository"
)

const (
	DatasetNormalizedRoutingKey = "dataset.normalized"
	RelationsBuiltRoutingKey    = "refund.relations.built"
	ScoringCompletedRoutingKey  = "refund.scoring.completed"
	PipelineFailedRoutingKey    = "pipeline.failed"
)

type PipelineEvent struct {
	DatasetID    string          `json:"datasetId"`
	JobID        string          `json:"jobId"`
	FailedStep   string          `json:"failedStep,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Stage        string          `json:"stage,omitempty"`
	Message      string          `json:"message,omitempty"`
	Timestamp    json.RawMessage `json:"timestamp,omitempty"`
	PublishedAt  string          `json:"publishedAt,omitempty"`
}

type PermanentEventError struct{ Message string }

func (e *PermanentEventError) Error() string   { return e.Message }
func (e *PermanentEventError) Permanent() bool { return true }

type eventTransitionRepository interface {
	TransitionFromEvent(context.Context, uuid.UUID, uuid.UUID, string, []string, string, string, string, string, time.Time) (bool, error)
}

func (s *Service) HandlePipelineEvent(ctx context.Context, routingKey string, body []byte) error {
	var event PipelineEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return &PermanentEventError{Message: "invalid pipeline event JSON"}
	}
	datasetID, err := uuid.Parse(strings.TrimSpace(event.DatasetID))
	if err != nil {
		return &PermanentEventError{Message: "pipeline event has invalid datasetId"}
	}
	jobID, err := uuid.Parse(strings.TrimSpace(event.JobID))
	if err != nil {
		return &PermanentEventError{Message: "pipeline event has invalid or missing jobId"}
	}
	repo, ok := s.repo.(eventTransitionRepository)
	if !ok {
		return fmt.Errorf("repository does not support event transitions")
	}
	now := time.Now().UTC()
	var allowed []string
	var status, currentStep, failedStage, message string
	switch routingKey {
	case DatasetNormalizedRoutingKey:
		allowed = []string{domain.AnalysisStatusNormalizing}
		status = domain.AnalysisStatusBuildingRelations
		currentStep = domain.AnalysisStatusBuildingRelations
	case RelationsBuiltRoutingKey:
		allowed = []string{domain.AnalysisStatusNormalized, domain.AnalysisStatusBuildingRelations}
		status = domain.AnalysisStatusScoring
		currentStep = domain.AnalysisStatusScoring
	case ScoringCompletedRoutingKey:
		allowed = []string{domain.AnalysisStatusScoring}
		status = domain.AnalysisStatusCompleted
		currentStep = domain.AnalysisStatusCompleted
	case PipelineFailedRoutingKey:
		status = domain.AnalysisStatusFailed
		failedStep := event.FailedStep
		if strings.TrimSpace(failedStep) == "" {
			failedStep = event.Stage
		}
		failureMessage := event.ErrorMessage
		if strings.TrimSpace(failureMessage) == "" {
			failureMessage = event.Message
		}
		failedStage = normalizeFailedStage(failedStep)
		switch failedStage {
		case domain.AnalysisStatusUploaded:
			allowed = []string{domain.AnalysisStatusUploaded, domain.AnalysisStatusNormalizing}
		case domain.AnalysisStatusNormalizing:
			allowed = []string{domain.AnalysisStatusNormalizing}
		case domain.AnalysisStatusBuildingRelations:
			allowed = []string{domain.AnalysisStatusNormalized, domain.AnalysisStatusBuildingRelations}
		case domain.AnalysisStatusScoring:
			allowed = []string{domain.AnalysisStatusScoring}
		default:
			allowed = []string{domain.AnalysisStatusUploaded, domain.AnalysisStatusNormalizing, domain.AnalysisStatusNormalized, domain.AnalysisStatusBuildingRelations, domain.AnalysisStatusScoring}
		}
		currentStep = failedStage
		message = readablePipelineError(failureMessage, failedStage)
	default:
		return &PermanentEventError{Message: "unsupported pipeline routing key"}
	}
	changed, err := repo.TransitionFromEvent(ctx, jobID, datasetID, routingKey, allowed, status, currentStep, failedStage, message, now)
	if errors.Is(err, repository.ErrCorrelation) {
		return &PermanentEventError{Message: "pipeline event datasetId/jobId correlation mismatch"}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return &PermanentEventError{Message: "pipeline event references an unknown job"}
	}
	if err != nil {
		return fmt.Errorf("apply pipeline event: %w", err)
	}
	if s.logger != nil {
		s.logger.Info("pipeline event processed", "routingKey", routingKey, "datasetId", datasetID, "jobId", jobID, "changed", changed, "status", status)
	}
	return nil
}

func normalizeFailedStage(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "UPLOAD", "UPLOADED":
		return domain.AnalysisStatusUploaded
	case "NORMALIZATION", "NORMALIZING", "NORMALIZED":
		return domain.AnalysisStatusNormalizing
	case "RELATIONS", "BUILDING_RELATIONS":
		return domain.AnalysisStatusBuildingRelations
	case "SCORING":
		return domain.AnalysisStatusScoring
	}
	return domain.AnalysisStatusFailed
}

func readablePipelineError(message, stage string) string {
	message = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < ' ' && r != '\t' {
			return -1
		}
		return r
	}, message))
	if message == "" {
		message = fmt.Sprintf("Pipeline failed during %s.", strings.ToLower(stage))
	}
	runes := []rune(message)
	if len(runes) > 500 {
		message = string(runes[:500])
	}
	return message
}
