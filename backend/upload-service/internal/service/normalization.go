package service

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const normalizedSchemaVersion = "refund-normalized.v1"

var normalizedColumns = []string{
	"order_id", "customer_id", "return_id", "support_agent_id",
	"order_amount", "refund_amount", "product_category", "return_reason",
	"evidence_provided", "decision", "manual_override",
	"decision_time_minutes", "timestamp",
}

type normalizedDatasetEvent struct {
	DatasetID     string `json:"datasetId"`
	JobID         string `json:"jobId"`
	RecordsPath   string `json:"recordsPath"`
	RecordCount   int    `json:"recordCount"`
	SchemaVersion string `json:"schemaVersion"`
	PublishedAt   string `json:"publishedAt"`
}

type normalizationFailedEvent struct {
	DatasetID string `json:"datasetId"`
	JobID     string `json:"jobId"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	FailedAt  string `json:"failedAt"`
}

func (s *Service) HandleRabbitEvent(ctx context.Context, routingKey string, body []byte) error {
	if routingKey == DatasetUploadedRoutingKey {
		return s.HandleDatasetUploaded(ctx, body)
	}
	return s.HandlePipelineEvent(ctx, routingKey, body)
}

func (s *Service) HandleDatasetUploaded(ctx context.Context, body []byte) error {
	var event datasetUploadedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return &PermanentEventError{Message: "invalid dataset.uploaded event JSON"}
	}
	if _, err := uuid.Parse(strings.TrimSpace(event.DatasetID)); err != nil {
		return &PermanentEventError{Message: "dataset.uploaded event has invalid datasetId"}
	}
	if _, err := uuid.Parse(strings.TrimSpace(event.JobID)); err != nil {
		return &PermanentEventError{Message: "dataset.uploaded event has invalid jobId"}
	}
	if strings.TrimSpace(event.FilePath) == "" {
		return s.publishNormalizationFailure(ctx, event, errors.New("source filePath is missing"))
	}

	recordsPath, recordCount, err := s.normalizeUploadedObject(ctx, event)
	if err != nil {
		return s.publishNormalizationFailure(ctx, event, err)
	}

	normalized := normalizedDatasetEvent{
		DatasetID: event.DatasetID, JobID: event.JobID, RecordsPath: recordsPath,
		RecordCount: recordCount, SchemaVersion: normalizedSchemaVersion,
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.publisher.Publish(ctx, DatasetNormalizedRoutingKey, normalized); err != nil {
		return fmt.Errorf("publish dataset.normalized: %w", err)
	}
	if s.logger != nil {
		s.logger.Info("dataset normalized", "datasetId", event.DatasetID, "jobId", event.JobID, "recordsPath", recordsPath, "recordCount", recordCount)
	}
	return nil
}

func (s *Service) normalizeUploadedObject(ctx context.Context, event datasetUploadedEvent) (string, int, error) {
	source, err := s.store.Get(ctx, event.FilePath)
	if err != nil {
		return "", 0, fmt.Errorf("load source dataset: %w", err)
	}
	defer source.Close()

	if err := os.MkdirAll(s.normalizedDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create normalized dataset directory: %w", err)
	}
	temporary, err := os.CreateTemp(s.normalizedDir, event.DatasetID+"-*.csv")
	if err != nil {
		return "", 0, fmt.Errorf("create normalized dataset: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	reader := csv.NewReader(bufio.NewReader(&contextReader{ctx: ctx, reader: source}))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return "", 0, fmt.Errorf("read source CSV header: %w", err)
	}
	headers = trimRecord(headers)
	columns, err := validateHeaders(headers)
	if err != nil {
		return "", 0, err
	}

	writer := csv.NewWriter(temporary)
	if err := writer.Write(normalizedColumns); err != nil {
		return "", 0, fmt.Errorf("write normalized CSV header: %w", err)
	}
	recordCount := 0
	seenReturns := make(map[string]string)
	for line := 2; ; line++ {
		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", 0, fmt.Errorf("read source CSV line %d: %w", line, readErr)
		}
		if len(row) != len(headers) {
			return "", 0, fmt.Errorf("source CSV line %d has %d columns, expected %d", line, len(row), len(headers))
		}
		normalized, normalizeErr := normalizeCSVRow(trimRecord(row), columns)
		if normalizeErr != nil {
			return "", 0, fmt.Errorf("normalize source CSV line %d: %w", line, normalizeErr)
		}
		returnID := normalized[2]
		signature := strings.Join(normalized, "\x1f")
		if previous, exists := seenReturns[returnID]; exists {
			if previous == signature {
				continue
			}
			return "", 0, fmt.Errorf("normalize source CSV line %d: conflicting duplicate return_id %q", line, returnID)
		}
		seenReturns[returnID] = signature
		if err := writer.Write(normalized); err != nil {
			return "", 0, fmt.Errorf("write normalized CSV line %d: %w", line, err)
		}
		recordCount++
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", 0, fmt.Errorf("flush normalized CSV: %w", err)
	}
	if recordCount == 0 {
		return "", 0, errors.New("normalized dataset is empty")
	}
	if err := temporary.Close(); err != nil {
		return "", 0, fmt.Errorf("close normalized CSV: %w", err)
	}

	finalPath := filepath.Join(s.normalizedDir, event.DatasetID+".csv")
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", 0, fmt.Errorf("commit normalized dataset: %w", err)
	}
	committed = true
	return finalPath, recordCount, nil
}

func normalizeCSVRow(row []string, columns map[string]int) ([]string, error) {
	value := func(name string) string {
		index, ok := columns[name]
		if !ok || index >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[index])
	}

	orderAmount, err := parseFlexibleFloat(value("order_amount"))
	if err != nil {
		return nil, fmt.Errorf("invalid order_amount: %w", err)
	}
	refundAmount, err := parseFlexibleFloat(value("refund_amount"))
	if err != nil {
		return nil, fmt.Errorf("invalid refund_amount: %w", err)
	}
	minutes := 60
	if raw := value("decision_time_minutes"); raw != "" {
		parsed, parseErr := parseFlexibleFloat(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid decision_time_minutes: %w", parseErr)
		}
		minutes = int(parsed)
	}

	return []string{
		rewriteIdentifier(value("order_id"), "purchase_", "order_"),
		rewriteIdentifier(value("customer_id"), "buyer_", "customer_"),
		rewriteIdentifier(value("return_id"), "refund_req_", "return_"),
		value("support_agent_id"),
		strconv.FormatFloat(orderAmount, 'f', 2, 64),
		strconv.FormatFloat(refundAmount, 'f', 2, 64),
		strings.ToLower(value("product_category")),
		value("return_reason"),
		strconv.FormatBool(normalizeBoolean(value("evidence_provided"))),
		normalizeDecision(value("decision")),
		strconv.FormatBool(normalizeBoolean(value("manual_override"))),
		strconv.Itoa(minutes),
		normalizeTimestamp(value("timestamp")),
	}, nil
}

func rewriteIdentifier(value, sourcePrefix, targetPrefix string) string {
	if strings.HasPrefix(value, sourcePrefix) {
		return targetPrefix + strings.TrimPrefix(value, sourcePrefix)
	}
	return value
}

func normalizeBoolean(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "y", "1", "t":
		return true
	default:
		return false
	}
}

func normalizeDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "approved", "approve", "ok", "accepted", "accept", "yes":
		return "APPROVED"
	case "declined", "decline", "rejected", "reject", "denied", "deny", "no":
		return "DECLINED"
	default:
		return strings.ToUpper(strings.TrimSpace(value))
	}
}

func normalizeTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
	}
	for _, layout := range []string{
		time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04",
		"01/02/2006 15:04", "01/02/2006 15:04:05",
		"02.01.2006 15:04", "02.01.2006 15:04:05",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return value
}

func (s *Service) publishNormalizationFailure(ctx context.Context, event datasetUploadedEvent, cause error) error {
	failure := normalizationFailedEvent{
		DatasetID: event.DatasetID, JobID: event.JobID, Stage: "NORMALIZATION",
		Message: cause.Error(), FailedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.publisher.Publish(ctx, PipelineFailedRoutingKey, failure); err != nil {
		return fmt.Errorf("normalization failed (%v) and failure event could not be published: %w", cause, err)
	}
	if s.logger != nil {
		s.logger.Error("dataset normalization failed", "datasetId", event.DatasetID, "jobId", event.JobID, "error", cause)
	}
	return nil
}
