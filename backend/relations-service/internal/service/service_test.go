package service

import (
	"context"
	"testing"

	"relations-service/internal/domain"
)

func TestCalculateCustomerReturnCount(t *testing.T) {
	svc := NewServiceWithRecords(nil, featureTestRecords())

	features, err := svc.GetReturnFeatures("return_a")
	if err != nil {
		t.Fatalf("GetReturnFeatures returned error: %v", err)
	}

	if features.Features.CustomerReturnCount != 3 {
		t.Fatalf("customerReturnCount = %d, want 3", features.Features.CustomerReturnCount)
	}
}

func TestCalculateAgentApprovalRate(t *testing.T) {
	svc := NewServiceWithRecords(nil, featureTestRecords())

	features, err := svc.GetReturnFeatures("return_a")
	if err != nil {
		t.Fatalf("GetReturnFeatures returned error: %v", err)
	}

	if features.Features.AgentApprovalRate != 0.67 {
		t.Fatalf("agentApprovalRate = %.2f, want 0.67", features.Features.AgentApprovalRate)
	}
}

func TestCalculateCustomerAgentPairCount(t *testing.T) {
	svc := NewServiceWithRecords(nil, featureTestRecords())

	features, err := svc.GetReturnFeatures("return_a")
	if err != nil {
		t.Fatalf("GetReturnFeatures returned error: %v", err)
	}

	if features.Features.CustomerAgentPairCount != 2 {
		t.Fatalf("customerAgentPairCount = %d, want 2", features.Features.CustomerAgentPairCount)
	}
}

func TestCalculateClusterSizeFallback(t *testing.T) {
	svc := NewServiceWithRecords(nil, featureTestRecords())

	features, err := svc.GetReturnFeatures("return_a")
	if err != nil {
		t.Fatalf("GetReturnFeatures returned error: %v", err)
	}

	if features.Features.ClusterSize != 3 {
		t.Fatalf("clusterSize = %d, want 3", features.Features.ClusterSize)
	}
}

func TestStrongestRelationAndTopRelatedReturnsExplainRisk(t *testing.T) {
	svc := NewServiceWithRecords(nil, featureTestRecords())

	features, err := svc.GetReturnFeatures("return_a")
	if err != nil {
		t.Fatalf("GetReturnFeatures returned error: %v", err)
	}

	if features.Features.StrongestRelationType == "" {
		t.Fatal("strongestRelationType should not be empty")
	}
	if len(features.Features.TopRelatedReturns) == 0 {
		t.Fatal("topRelatedReturns should not be empty")
	}
	if features.Features.ExplanationSummary == "" {
		t.Fatal("explanationSummary should not be empty")
	}
	if len(features.Features.ExplanationSignals) == 0 {
		t.Fatal("explanationSignals should not be empty")
	}
}

func TestDatasetAwareFeatures(t *testing.T) {
	svc := NewServiceWithRecords(nil, featureTestRecords())

	features, err := svc.GetReturnFeaturesForDataset("demo", "return_a")
	if err != nil {
		t.Fatalf("GetReturnFeaturesForDataset returned error: %v", err)
	}

	if features.ReturnID != "return_a" {
		t.Fatalf("returnId = %s, want return_a", features.ReturnID)
	}
}

func TestRebuildDatasetStats(t *testing.T) {
	svc := NewServiceWithRecords(nil, featureTestRecords())

	rebuild, err := svc.RebuildDataset(context.Background(), "demo")
	if err != nil {
		t.Fatalf("RebuildDataset returned error: %v", err)
	}

	if rebuild.FeaturesCount != 4 {
		t.Fatalf("featuresCount = %d, want 4", rebuild.FeaturesCount)
	}
	if rebuild.RelationsCount != 28 {
		t.Fatalf("relationsCount = %d, want 28", rebuild.RelationsCount)
	}
}

func TestCleanRefundDatasetSupportsAllDemoReturnIDs(t *testing.T) {
	records, err := loadRecords(Options{
		DatasetID:   "demo",
		DatasetPath: "../../../../data/clean_refund_dataset.csv",
	})
	if err != nil {
		t.Fatalf("loadRecords returned error: %v", err)
	}

	svc := NewServiceWithRecords(nil, records)
	for i := 3001; i <= 3045; i++ {
		returnID := "return_" + itoa(i)
		features, err := svc.GetReturnFeaturesForDataset("demo", returnID)
		if err != nil {
			t.Fatalf("GetReturnFeaturesForDataset(%s) returned error: %v", returnID, err)
		}
		if features.Features.CustomerReturnCount == 0 {
			t.Fatalf("%s customerReturnCount is zero", returnID)
		}
	}
}

func TestCleanRefundDatasetHighRiskDemoCaseHasBusinessSignals(t *testing.T) {
	records, err := loadRecords(Options{
		DatasetID:   "demo",
		DatasetPath: "../../../../data/clean_refund_dataset.csv",
	})
	if err != nil {
		t.Fatalf("loadRecords returned error: %v", err)
	}

	svc := NewServiceWithRecords(nil, records)
	features, err := svc.GetReturnFeaturesForDataset("demo", "return_3041")
	if err != nil {
		t.Fatalf("GetReturnFeaturesForDataset returned error: %v", err)
	}

	if features.Features.CustomerReturnCount < 5 {
		t.Fatalf("customerReturnCount = %d, want at least 5", features.Features.CustomerReturnCount)
	}
	if features.Features.AgentApprovalRate < 0.9 {
		t.Fatalf("agentApprovalRate = %.2f, want at least 0.9", features.Features.AgentApprovalRate)
	}
	if features.Features.CustomerAgentPairCount < 5 {
		t.Fatalf("customerAgentPairCount = %d, want at least 5", features.Features.CustomerAgentPairCount)
	}
	if len(features.Features.ExplanationSignals) < 3 {
		t.Fatalf("explanationSignals length = %d, want at least 3", len(features.Features.ExplanationSignals))
	}
}

func featureTestRecords() []domain.NormalizedReturnRecord {
	return []domain.NormalizedReturnRecord{
		{
			DatasetID:       "demo",
			ReturnID:        "return_a",
			CustomerID:      "customer_1",
			OrderID:         "order_1",
			SupportAgentID:  "agent_1",
			ProductCategory: "electronics",
			ReturnReason:    "damaged_item",
			DecisionID:      "decision_a",
			DecisionStatus:  approvedDecision,
			RefundAmount:    80,
			OrderAmount:     100,
		},
		{
			DatasetID:       "demo",
			ReturnID:        "return_b",
			CustomerID:      "customer_1",
			OrderID:         "order_2",
			SupportAgentID:  "agent_1",
			ProductCategory: "electronics",
			ReturnReason:    "damaged_item",
			DecisionID:      "decision_b",
			DecisionStatus:  approvedDecision,
			RefundAmount:    70,
			OrderAmount:     100,
		},
		{
			DatasetID:       "demo",
			ReturnID:        "return_c",
			CustomerID:      "customer_1",
			OrderID:         "order_3",
			SupportAgentID:  "agent_2",
			ProductCategory: "books",
			ReturnReason:    "changed_mind",
			DecisionID:      "decision_c",
			DecisionStatus:  "REJECTED",
			RefundAmount:    0,
			OrderAmount:     50,
		},
		{
			DatasetID:       "demo",
			ReturnID:        "return_d",
			CustomerID:      "customer_2",
			OrderID:         "order_4",
			SupportAgentID:  "agent_1",
			ProductCategory: "electronics",
			ReturnReason:    "damaged_item",
			DecisionID:      "decision_d",
			DecisionStatus:  "REJECTED",
			RefundAmount:    0,
			OrderAmount:     120,
		},
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	digits := make([]byte, 0, 4)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}

	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}

	return string(digits)
}
