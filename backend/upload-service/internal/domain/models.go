package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	AnalysisStatusUploaded          = "UPLOADED"
	AnalysisStatusNormalizing       = "NORMALIZING"
	AnalysisStatusNormalized        = "NORMALIZED"
	AnalysisStatusBuildingRelations = "BUILDING_RELATIONS"
	AnalysisStatusScoring           = "SCORING"
	AnalysisStatusCompleted         = "COMPLETED"
	AnalysisStatusFailed            = "FAILED"
)

type Dataset struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	OriginalFilename string     `json:"originalFilename"`
	FileType         string     `json:"fileType"`
	Status           string     `json:"status"`
	UploadedAt       time.Time  `json:"uploadedAt"`
	ArchivedAt       *time.Time `json:"archivedAt,omitempty"`
	SizeBytes        int64      `json:"sizeBytes"`
	RowCount         int        `json:"rowCount"`
	Warnings         []string   `json:"warnings"`
}

type UploadedFile struct {
	ID         uuid.UUID `json:"id"`
	DatasetID  uuid.UUID `json:"datasetId"`
	FilePath   string    `json:"filePath"`
	FileType   string    `json:"fileType"`
	UploadedAt time.Time `json:"uploadedAt"`
}

type AnalysisJob struct {
	ID           uuid.UUID  `json:"id"`
	DatasetID    uuid.UUID  `json:"datasetId"`
	Status       string     `json:"status"`
	CurrentStep  string     `json:"currentStep"`
	FailedStage  string     `json:"failedStage,omitempty"`
	Error        string     `json:"errorMessage,omitempty"`
	ResultReady  bool       `json:"resultReady"`
	RetryOfJobID *uuid.UUID `json:"retryOfJobId,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
	FailedAt     *time.Time `json:"failedAt,omitempty"`
}

type AuditEvent struct {
	ID         int64      `json:"id"`
	DatasetID  uuid.UUID  `json:"datasetId"`
	JobID      *uuid.UUID `json:"jobId,omitempty"`
	EventType  string     `json:"eventType"`
	FromStatus string     `json:"fromStatus,omitempty"`
	ToStatus   string     `json:"toStatus,omitempty"`
	Message    string     `json:"message,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// Domain entities for refund detection (kept minimal for upload-service)
type Customer struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Order struct {
	ID         uuid.UUID `json:"id"`
	CustomerID uuid.UUID `json:"customerId"`
	Amount     float64   `json:"amount"`
	Category   string    `json:"category"`
	CreatedAt  time.Time `json:"createdAt"`
}

type ReturnRequest struct {
	ID          uuid.UUID `json:"id"`
	OrderID     uuid.UUID `json:"orderId"`
	Reason      string    `json:"reason"`
	Amount      float64   `json:"amount"`
	Evidence    bool      `json:"evidence"`
	RequestedAt time.Time `json:"requestedAt"`
}

type SupportAgent struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type SupportDecision struct {
	ID              uuid.UUID `json:"id"`
	ReturnRequestID uuid.UUID `json:"returnRequestId"`
	AgentID         uuid.UUID `json:"agentId"`
	Decision        string    `json:"decision"`
	RefundAmount    float64   `json:"refundAmount"`
	DecidedAt       time.Time `json:"decidedAt"`
}

type RefundApprovalRisk struct {
	ApprovalID uuid.UUID `json:"approvalId"`
	Score      float64   `json:"score"`
	Reason     string    `json:"reason"`
}
