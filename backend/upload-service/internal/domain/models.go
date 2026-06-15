package domain

import (
	"time"

	"github.com/google/uuid"
)

type Dataset struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	OriginalFilename string    `json:"originalFilename"`
	FileType         string    `json:"fileType"`
	Status           string    `json:"status"`
	UploadedAt       time.Time `json:"uploadedAt"`
}

type UploadedFile struct {
	ID         uuid.UUID `json:"id"`
	DatasetID  uuid.UUID `json:"datasetId"`
	FilePath   string    `json:"filePath"`
	FileType   string    `json:"fileType"`
	UploadedAt time.Time `json:"uploadedAt"`
}

type AnalysisJob struct {
	ID          uuid.UUID `json:"id"`
	DatasetID   uuid.UUID `json:"datasetId"`
	Status      string    `json:"status"`
	CurrentStep string    `json:"currentStep"`
	Error       string    `json:"errorMessage,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
