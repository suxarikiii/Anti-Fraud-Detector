package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"upload-service/internal/domain"
)

type ValidationIssue struct {
	Row     int    `json:"row,omitempty"`
	Column  string `json:"column,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type UploadResult struct {
	DatasetID uuid.UUID         `json:"-"`
	JobID     uuid.UUID         `json:"-"`
	Filename  string            `json:"filename"`
	RowCount  int               `json:"rowCount"`
	SizeBytes int64             `json:"sizeBytes"`
	Warnings  []ValidationIssue `json:"warnings"`
}

type validationResult struct {
	Rows     int
	Warnings []ValidationIssue
}

type atomicUploadRepository interface {
	CreateDatasetWithFileAndJob(context.Context, uuid.UUID, uuid.UUID, string, string, string, string, string, int64, int, []string, time.Time) error
}

type deletingStore interface {
	Delete(context.Context, string) error
}

func (s *Service) UploadDatasetDetailed(ctx context.Context, reader io.Reader, declaredSize int64, originalFilename, declaredContentType string) (*UploadResult, error) {
	filename := sanitizeFilename(originalFilename)
	if !strings.EqualFold(filepath.Ext(filename), ".csv") {
		return nil, invalidIssue("INVALID_EXTENSION", "uploaded file must have .csv extension")
	}
	if declaredSize > s.maxFileSize {
		return nil, invalidIssue("FILE_TOO_LARGE", fmt.Sprintf("uploaded file exceeds %d byte limit", s.maxFileSize))
	}

	temporary, err := os.CreateTemp("", "upload-service-*.csv")
	if err != nil {
		return nil, fmt.Errorf("create upload spool: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() { _ = temporary.Close(); _ = os.Remove(temporaryName) }()

	written, err := io.Copy(temporary, io.LimitReader(&contextReader{ctx: ctx, reader: reader}, s.maxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if written > s.maxFileSize {
		return nil, invalidIssue("FILE_TOO_LARGE", fmt.Sprintf("uploaded file exceeds %d byte limit", s.maxFileSize))
	}
	if written == 0 {
		return nil, invalidIssue("EMPTY_FILE", "uploaded CSV is empty")
	}
	if _, err = temporary.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind upload: %w", err)
	}

	probe := make([]byte, 512)
	n, probeErr := io.ReadFull(temporary, probe)
	if probeErr != nil && !errors.Is(probeErr, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("inspect upload: %w", probeErr)
	}
	probe = probe[:n]
	if len(bytes.TrimSpace(probe)) == 0 && written <= 512 {
		return nil, invalidIssue("EMPTY_FILE", "uploaded CSV is empty")
	}
	if strings.ContainsRune(string(probe), '\x00') {
		return nil, invalidIssue("INVALID_MIME", "uploaded file must be textual CSV")
	}
	detected := http.DetectContentType(probe)
	if !isCSVContentType(declaredContentType) || !isCSVContentType(detected) {
		return nil, invalidIssue("INVALID_MIME", "uploaded file must have a CSV or plain-text content type")
	}
	if _, err = temporary.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind upload: %w", err)
	}

	validation, err := validateCSVStream(temporary, s.maxRows, s.maxErrors)
	if err != nil {
		return nil, err
	}
	if _, err = temporary.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind upload: %w", err)
	}

	datasetID, jobID := uuid.New(), uuid.New()
	objectName := fmt.Sprintf("datasets/%s.csv", datasetID)
	if err = s.store.Put(ctx, objectName, temporary, written, "text/csv"); err != nil {
		return nil, fmt.Errorf("upload object: %w", err)
	}
	cleanup := func() {
		if store, ok := s.store.(deletingStore); ok {
			if deleteErr := store.Delete(context.Background(), objectName); deleteErr != nil && s.logger != nil {
				s.logger.Error("failed to compensate object upload", "datasetId", datasetID, "error", deleteErr)
			}
		}
	}

	now := time.Now().UTC()
	warningMessages := make([]string, len(validation.Warnings))
	for i, warning := range validation.Warnings {
		warningMessages[i] = warning.Message
	}
	if repo, ok := s.repo.(atomicUploadRepository); ok {
		err = repo.CreateDatasetWithFileAndJob(ctx, datasetID, jobID, filename, filename, domain.AnalysisStatusUploaded, objectName, "csv", written, validation.Rows, warningMessages, now)
	} else {
		err = s.repo.CreateDatasetWithFile(ctx, datasetID, filename, filename, domain.AnalysisStatusUploaded, objectName, "csv", now, now)
		if err == nil {
			err = s.repo.CreateAnalysisJob(ctx, jobID, datasetID, domain.AnalysisStatusUploaded, domain.AnalysisStatusUploaded, now, now)
		}
	}
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("create upload records: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("dataset uploaded", "datasetId", datasetID, "jobId", jobID, "filename", filename, "sizeBytes", written, "rowCount", validation.Rows, "warningCount", len(validation.Warnings))
	}
	return &UploadResult{DatasetID: datasetID, JobID: jobID, Filename: filename, RowCount: validation.Rows, SizeBytes: written, Warnings: validation.Warnings}, nil
}

func validateCSVStream(reader io.Reader, maxRows, maxErrors int) (*validationResult, error) {
	csvReader := csv.NewReader(bufio.NewReader(reader))
	csvReader.TrimLeadingSpace = true
	csvReader.FieldsPerRecord = -1
	headers, err := csvReader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, invalidIssue("EMPTY_FILE", "uploaded CSV is empty")
		}
		return nil, invalidIssue("INVALID_HEADER", "failed to read CSV headers")
	}
	headers = trimRecord(headers)
	columns, err := validateHeaders(headers)
	if err != nil {
		return nil, err
	}

	issues := make([]ValidationIssue, 0)
	warnings := make([]ValidationIssue, 0)
	type seenReturn struct {
		row       int
		signature string
	}
	returnIDs := make(map[string]seenReturn)
	rows := 0
	records := 0
	for {
		record, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		records++
		rowNumber := records + 1
		if readErr != nil {
			issues = appendIssue(issues, maxErrors, ValidationIssue{Row: rowNumber, Code: "MALFORMED_ROW", Message: "row cannot be parsed as CSV"})
			if len(issues) >= maxErrors {
				break
			}
			continue
		}
		rows++
		if rows > maxRows {
			issues = appendIssue(issues, maxErrors, ValidationIssue{Row: rowNumber, Code: "TOO_MANY_ROWS", Message: fmt.Sprintf("CSV exceeds %d row limit", maxRows)})
			break
		}
		record = trimRecord(record)
		if len(record) != len(headers) {
			issues = appendIssue(issues, maxErrors, ValidationIssue{Row: rowNumber, Code: "ROW_WIDTH", Message: fmt.Sprintf("row has %d columns; expected %d", len(record), len(headers))})
			continue
		}
		for _, name := range requiredSemanticColumns {
			idx := columns[name]
			if strings.TrimSpace(record[idx]) == "" {
				issues = appendIssue(issues, maxErrors, ValidationIssue{Row: rowNumber, Column: name, Code: "EMPTY_REQUIRED", Message: fmt.Sprintf("row %d has empty required field %s", rowNumber, name)})
			}
		}
		returnID := strings.TrimSpace(record[columns["return_id"]])
		if returnID != "" {
			signature := strings.Join(record, "\x1f")
			if first, exists := returnIDs[returnID]; exists {
				if first.signature == signature {
					warnings = appendIssue(warnings, maxErrors, ValidationIssue{Row: rowNumber, Column: "return_id", Code: "DUPLICATE_ROW", Message: fmt.Sprintf("row %d duplicates row %d and will be removed during normalization", rowNumber, first.row)})
				} else {
					issues = appendIssue(issues, maxErrors, ValidationIssue{Row: rowNumber, Column: "return_id", Code: "DUPLICATE_RETURN_ID", Message: fmt.Sprintf("conflicting duplicate return_id %q; first seen on row %d", returnID, first.row)})
				}
			} else {
				returnIDs[returnID] = seenReturn{row: rowNumber, signature: signature}
			}
		}
		validateFiniteNumber(record, columns, "order_amount", rowNumber, true, &issues, maxErrors)
		validateFiniteNumber(record, columns, "refund_amount", rowNumber, false, &issues, maxErrors)
		validateFiniteNumber(record, columns, "decision_time_minutes", rowNumber, false, &issues, maxErrors)
		if idx, ok := columns["refund_amount"]; ok && strings.TrimSpace(record[idx]) != "" {
			if value, parseErr := parseFlexibleFloat(record[idx]); parseErr == nil && value == 0 {
				warnings = appendIssue(warnings, maxErrors, ValidationIssue{Row: rowNumber, Column: "refund_amount", Code: "ZERO_REFUND_AMOUNT", Message: fmt.Sprintf("row %d has zero refund_amount", rowNumber)})
			}
		}
		if idx := columns["decision"]; idx < len(record) && !validDecision(record[idx]) {
			issues = appendIssue(issues, maxErrors, ValidationIssue{Row: rowNumber, Column: "decision", Code: "INVALID_DECISION", Message: fmt.Sprintf("row %d has unsupported decision %q", rowNumber, record[idx])})
		}
		if timestampErr := validateTimestampField(record, columns, rowNumber); timestampErr != nil {
			issues = appendIssue(issues, maxErrors, ValidationIssue{Row: rowNumber, Column: "timestamp", Code: "INVALID_TIMESTAMP", Message: timestampErr.Error()})
		}
		if idx, ok := columns["decision_time_minutes"]; ok && strings.TrimSpace(record[idx]) == "" {
			warnings = appendIssue(warnings, maxErrors, ValidationIssue{Row: rowNumber, Column: "decision_time_minutes", Code: "MISSING_OPTIONAL", Message: fmt.Sprintf("row %d has no decision_time_minutes", rowNumber)})
		}
		if len(issues) >= maxErrors {
			break
		}
	}
	if rows == 0 && len(issues) == 0 {
		issues = append(issues, ValidationIssue{Code: "NO_ROWS", Message: "uploaded CSV must contain at least one data row"})
	}
	if len(issues) > 0 {
		return nil, &InvalidUploadError{Message: fmt.Sprintf("CSV validation failed with %d error(s)", len(issues)), Errors: issues, Warnings: warnings}
	}
	return &validationResult{Rows: rows, Warnings: warnings}, nil
}

func validateFiniteNumber(record []string, columns map[string]int, name string, row int, positive bool, issues *[]ValidationIssue, maxErrors int) {
	idx, ok := columns[name]
	if !ok {
		return
	}
	raw := strings.TrimSpace(record[idx])
	if raw == "" {
		return
	}
	value, err := parseFlexibleFloat(raw)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		*issues = appendIssue(*issues, maxErrors, ValidationIssue{Row: row, Column: name, Code: "INVALID_NUMBER", Message: fmt.Sprintf("row %d has invalid numeric value for %s", row, name)})
		return
	}
	if positive && value <= 0 {
		*issues = appendIssue(*issues, maxErrors, ValidationIssue{Row: row, Column: name, Code: "NON_POSITIVE_AMOUNT", Message: fmt.Sprintf("row %d must have positive %s", row, name)})
	}
	if !positive && value < 0 {
		*issues = appendIssue(*issues, maxErrors, ValidationIssue{Row: row, Column: name, Code: "NEGATIVE_VALUE", Message: fmt.Sprintf("row %d must not have negative %s", row, name)})
	}
}

func appendIssue(issues []ValidationIssue, max int, issue ValidationIssue) []ValidationIssue {
	if len(issues) < max {
		return append(issues, issue)
	}
	return issues
}
func invalidIssue(code, message string) *InvalidUploadError {
	issue := ValidationIssue{Code: code, Message: message}
	return &InvalidUploadError{Message: message, Errors: []ValidationIssue{issue}}
}
func validDecision(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "APPROVED", "APPROVE", "DECLINED", "DECLINE", "REJECTED", "REJECT":
		return true
	}
	return false
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		name = "dataset.csv"
	}
	runes := []rune(name)
	if len(runes) > 200 {
		ext := filepath.Ext(name)
		extRunes := []rune(ext)
		baseLimit := 200 - len(extRunes)
		if baseLimit < 1 {
			baseLimit = 1
			extRunes = nil
		}
		name = strings.TrimSpace(string(runes[:baseLimit])) + string(extRunes)
	}
	return name
}

func isCSVContentType(value string) bool {
	if value == "" {
		return true
	}
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch value {
	case "text/csv", "application/csv", "application/vnd.ms-excel", "text/plain", "application/octet-stream":
		return true
	}
	return false
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
}
