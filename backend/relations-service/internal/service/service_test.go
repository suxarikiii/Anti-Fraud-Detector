package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestProcessNormalizedDatasetKeepsDatasetsIndependent(t *testing.T) {
	publisher := &recordingPublisher{}
	svc := NewService(nil, Options{})
	firstPath := writeDatasetCSV(t, []csvRecord{
		{returnID: "return_a", customerID: "customer_a", orderID: "order_a", agentID: "agent_a", category: "electronics", reason: "damaged", decision: approvedDecision, refund: "80", orderAmount: "100", manualOverride: "true", minutes: "5"},
	})
	secondPath := writeDatasetCSV(t, []csvRecord{
		{returnID: "return_b", customerID: "customer_b", orderID: "order_b", agentID: "agent_b", category: "books", reason: "wrong_size", decision: approvedDecision, refund: "30", orderAmount: "60", manualOverride: "false", minutes: "8"},
		{returnID: "return_c", customerID: "customer_b", orderID: "order_c", agentID: "agent_b", category: "books", reason: "wrong_size", decision: "REJECTED", refund: "0", orderAmount: "90", manualOverride: "false", minutes: "10"},
	})
	svc.publisher = publisher

	if err := svc.ProcessNormalizedDataset(context.Background(), NormalizedDatasetEvent{DatasetID: "dataset_one", JobID: "job_one", RecordsPath: firstPath, RecordCount: 1, SchemaVersion: "refund-normalized.v1"}); err != nil {
		t.Fatalf("ProcessNormalizedDataset first returned error: %v", err)
	}
	if err := svc.ProcessNormalizedDataset(context.Background(), NormalizedDatasetEvent{DatasetID: "dataset_two", JobID: "job_two", RecordsPath: secondPath, RecordCount: 2, SchemaVersion: "refund-normalized.v1"}); err != nil {
		t.Fatalf("ProcessNormalizedDataset second returned error: %v", err)
	}

	first, err := svc.GetReturnFeaturesForDataset("dataset_one", "return_a")
	if err != nil {
		t.Fatalf("dataset_one features returned error: %v", err)
	}
	second, err := svc.GetReturnFeaturesForDataset("dataset_two", "return_b")
	if err != nil {
		t.Fatalf("dataset_two features returned error: %v", err)
	}
	if first.DatasetID != "dataset_one" || second.DatasetID != "dataset_two" {
		t.Fatalf("features should keep dataset scope, got %s and %s", first.DatasetID, second.DatasetID)
	}
	if second.Features.AgentApprovalRate != 0.5 {
		t.Fatalf("dataset_two approval rate = %.2f, want 0.50", second.Features.AgentApprovalRate)
	}
	if len(publisher.built) != 2 {
		t.Fatalf("published relations built events = %d, want 2", len(publisher.built))
	}
}

func TestProcessNormalizedDatasetReplacesOnlySelectedDataset(t *testing.T) {
	svc := NewService(nil, Options{})
	originalPath := writeDatasetCSV(t, []csvRecord{
		{returnID: "return_a", customerID: "customer_a", orderID: "order_a", agentID: "agent_a", category: "electronics", reason: "damaged", decision: approvedDecision, refund: "80", orderAmount: "100", manualOverride: "true", minutes: "5"},
	})
	replacementPath := writeDatasetCSV(t, []csvRecord{
		{returnID: "return_x", customerID: "customer_x", orderID: "order_x", agentID: "agent_x", category: "luxury", reason: "not_as_described", decision: approvedDecision, refund: "200", orderAmount: "250", manualOverride: "true", minutes: "4"},
	})
	otherPath := writeDatasetCSV(t, []csvRecord{
		{returnID: "return_b", customerID: "customer_b", orderID: "order_b", agentID: "agent_b", category: "books", reason: "wrong_size", decision: approvedDecision, refund: "30", orderAmount: "60", manualOverride: "false", minutes: "8"},
	})

	_ = svc.ProcessNormalizedDataset(context.Background(), NormalizedDatasetEvent{DatasetID: "dataset_one", RecordsPath: originalPath})
	_ = svc.ProcessNormalizedDataset(context.Background(), NormalizedDatasetEvent{DatasetID: "dataset_two", RecordsPath: otherPath})
	_ = svc.ProcessNormalizedDataset(context.Background(), NormalizedDatasetEvent{DatasetID: "dataset_one", RecordsPath: replacementPath})

	if _, err := svc.GetReturnFeaturesForDataset("dataset_one", "return_a"); !errors.Is(err, ErrReturnNotFound) {
		t.Fatalf("old return lookup error = %v, want ErrReturnNotFound", err)
	}
	if _, err := svc.GetReturnFeaturesForDataset("dataset_one", "return_x"); err != nil {
		t.Fatalf("replacement return lookup returned error: %v", err)
	}
	if _, err := svc.GetReturnFeaturesForDataset("dataset_two", "return_b"); err != nil {
		t.Fatalf("unrelated dataset lookup returned error: %v", err)
	}
}

func TestProcessNormalizedDatasetRejectsInvalidInputAndPublishesFailure(t *testing.T) {
	publisher := &recordingPublisher{}
	svc := NewService(publisher, Options{})
	duplicatePath := writeDatasetCSV(t, []csvRecord{
		{returnID: "return_dup", customerID: "customer_a", orderID: "order_a", agentID: "agent_a", category: "electronics", reason: "damaged", decision: approvedDecision, refund: "80", orderAmount: "100", manualOverride: "true", minutes: "5"},
		{returnID: "return_dup", customerID: "customer_b", orderID: "order_b", agentID: "agent_b", category: "books", reason: "wrong_size", decision: approvedDecision, refund: "30", orderAmount: "60", manualOverride: "false", minutes: "8"},
	})

	err := svc.ProcessNormalizedDataset(context.Background(), NormalizedDatasetEvent{DatasetID: "dataset_invalid", JobID: "job_invalid", RecordsPath: duplicatePath})
	if !errors.Is(err, ErrInvalidDataset) {
		t.Fatalf("ProcessNormalizedDataset error = %v, want ErrInvalidDataset", err)
	}
	if len(publisher.failed) != 1 {
		t.Fatalf("pipeline failed events = %d, want 1", len(publisher.failed))
	}
	if _, err := svc.GetReturnFeaturesForDataset("dataset_invalid", "return_dup"); !errors.Is(err, ErrDatasetNotFound) {
		t.Fatalf("invalid dataset should not be stored, got error %v", err)
	}
}

func TestGraphRelatedAndRankedAnalyticsAreDeterministic(t *testing.T) {
	svc := NewServiceWithRecords(nil, featureTestRecords())

	graph, err := svc.GetGraphProjection("demo", "return_a", 8)
	if err != nil {
		t.Fatalf("GetGraphProjection returned error: %v", err)
	}
	if len(graph.Nodes) == 0 || len(graph.Edges) == 0 {
		t.Fatalf("graph should contain nodes and edges: %+v", graph)
	}
	if graph.Limit != 8 {
		t.Fatalf("graph limit = %d, want 8", graph.Limit)
	}

	related, err := svc.GetRelatedReturns("demo", "return_a", 1)
	if err != nil {
		t.Fatalf("GetRelatedReturns returned error: %v", err)
	}
	if len(related.RelatedReturns) != 1 || !related.Truncated {
		t.Fatalf("related returns = %+v, want one truncated result", related)
	}
	if related.RelatedReturns[0].ReturnID != "return_b" {
		t.Fatalf("top related return = %s, want return_b", related.RelatedReturns[0].ReturnID)
	}

	ranked, err := svc.GetRankedAgents("demo", 2, "approvalRate")
	if err != nil {
		t.Fatalf("GetRankedAgents returned error: %v", err)
	}
	if len(ranked.Agents) == 0 || ranked.Agents[0].SupportAgentID != "agent_1" {
		t.Fatalf("ranked agents = %+v, want agent_1 first", ranked.Agents)
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

type recordingPublisher struct {
	built  []RelationsBuiltEvent
	failed []PipelineFailedEvent
}

func (p *recordingPublisher) PublishRelationsBuilt(_ context.Context, event RelationsBuiltEvent) error {
	p.built = append(p.built, event)
	return nil
}

func (p *recordingPublisher) PublishPipelineFailed(_ context.Context, event PipelineFailedEvent) error {
	p.failed = append(p.failed, event)
	return nil
}

type csvRecord struct {
	returnID       string
	customerID     string
	orderID        string
	agentID        string
	category       string
	reason         string
	decision       string
	refund         string
	orderAmount    string
	manualOverride string
	minutes        string
}

func writeDatasetCSV(t *testing.T, records []csvRecord) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "normalized.csv")
	content := "order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,product_category,return_reason,decision,manual_override,decision_time_minutes\n"
	for _, record := range records {
		content += record.orderID + "," + record.customerID + "," + record.returnID + "," + record.agentID + "," + record.orderAmount + "," + record.refund + "," + record.category + "," + record.reason + "," + record.decision + "," + record.manualOverride + "," + record.minutes + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test dataset: %v", err)
	}
	return path
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
