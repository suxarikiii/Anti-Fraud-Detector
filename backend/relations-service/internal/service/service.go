package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"relations-service/internal/domain"
)

const approvedDecision = "APPROVED"

var ErrReturnNotFound = errors.New("return not found")

type RelationsBuiltPublisher interface {
	PublishRelationsBuilt(ctx context.Context, event RelationsBuiltEvent) error
}

type Service struct {
	publisher RelationsBuiltPublisher
	records   []domain.NormalizedReturnRecord
}

type Options struct {
	DatasetID   string
	DatasetPath string
}

type NormalizedDatasetEvent struct {
	DatasetID   string `json:"datasetId"`
	JobID       string `json:"jobId"`
	RecordsPath string `json:"recordsPath,omitempty"`
	PublishedAt string `json:"publishedAt,omitempty"`
}

type RelationsBuiltEvent struct {
	DatasetID      string `json:"datasetId"`
	JobID          string `json:"jobId"`
	RelationsCount int    `json:"relationsCount"`
	FeaturesCount  int    `json:"featuresCount"`
	PublishedAt    string `json:"publishedAt"`
}

func NewService(publisher RelationsBuiltPublisher, options Options) *Service {
	records, err := loadRecords(options)
	if err != nil {
		records = fallbackTestDataset()
	}

	return &Service{
		publisher: publisher,
		records:   records,
	}
}

func NewServiceWithRecords(publisher RelationsBuiltPublisher, records []domain.NormalizedReturnRecord) *Service {
	return &Service{
		publisher: publisher,
		records:   records,
	}
}

func (s *Service) Health() map[string]string {
	return map[string]string{
		"status":  "UP",
		"service": "relations-service",
	}
}

func (s *Service) RebuildDataset(ctx context.Context, datasetID string) (*domain.DatasetRebuildResponse, error) {
	jobID := fmt.Sprintf("relations-job-%s", datasetID)
	stats := s.buildStats(datasetID)
	if err := s.publishRelationsBuilt(ctx, datasetID, jobID, stats); err != nil {
		return nil, err
	}

	return &domain.DatasetRebuildResponse{
		DatasetID:      datasetID,
		JobID:          jobID,
		Status:         "RELATIONS_REBUILD_STARTED",
		RelationsCount: stats.RelationsCount,
		FeaturesCount:  stats.FeaturesCount,
	}, nil
}

func (s *Service) ProcessNormalizedDataset(ctx context.Context, event NormalizedDatasetEvent) error {
	stats := s.buildStats(event.DatasetID)
	return s.publishRelationsBuilt(ctx, event.DatasetID, event.JobID, stats)
}

func (s *Service) GetReturnRelations(returnID string) (*domain.ReturnRelations, error) {
	record, ok := s.findRecord(returnID)
	if !ok {
		return nil, ErrReturnNotFound
	}

	decisionEdge := "DECLINED_RETURN"
	if record.DecisionStatus == approvedDecision {
		decisionEdge = "APPROVED_RETURN"
	}

	return &domain.ReturnRelations{
		ReturnID:        record.ReturnID,
		CustomerID:      record.CustomerID,
		OrderID:         record.OrderID,
		SupportAgentID:  record.SupportAgentID,
		ProductCategory: record.ProductCategory,
		Decision: domain.SupportDecision{
			DecisionID:     record.DecisionID,
			Status:         record.DecisionStatus,
			RefundAmount:   record.RefundAmount,
			ManualOverride: record.ManualOverride,
			DecisionTimeMs: record.DecisionTimeMs,
		},
		Relations: []domain.GraphRelation{
			{From: record.CustomerID, Type: "PLACED_ORDER", To: record.OrderID},
			{From: record.CustomerID, Type: "REQUESTED_RETURN", To: record.ReturnID},
			{From: record.OrderID, Type: "HAS_RETURN_REQUEST", To: record.ReturnID},
			{From: record.ReturnID, Type: "DECIDED_BY", To: record.SupportAgentID},
			{From: record.OrderID, Type: "HAS_CATEGORY", To: record.ProductCategory},
			{From: record.SupportAgentID, Type: "MADE_DECISION", To: record.DecisionID},
			{From: record.DecisionID, Type: decisionEdge, To: record.ReturnID},
		},
	}, nil
}

func (s *Service) GetCustomerHistory(customerID string) *domain.CustomerHistory {
	customerRecords := make([]domain.NormalizedReturnRecord, 0)
	agentPairs := map[string]int{}
	approvedCount := 0

	for _, record := range s.records {
		if record.CustomerID != customerID {
			continue
		}
		customerRecords = append(customerRecords, record)
		agentPairs[record.SupportAgentID]++
		if record.DecisionStatus == approvedDecision {
			approvedCount++
		}
	}

	recentReturns := make([]domain.ReturnListItem, 0, len(customerRecords))
	for _, record := range customerRecords {
		recentReturns = append(recentReturns, domain.ReturnListItem{
			ReturnID:       record.ReturnID,
			OrderID:        record.OrderID,
			Reason:         record.ReturnReason,
			Category:       record.ProductCategory,
			RefundAmount:   record.RefundAmount,
			DecisionStatus: record.DecisionStatus,
			SupportAgentID: record.SupportAgentID,
		})
	}

	linkedAgents := make([]domain.LinkedAgent, 0, len(agentPairs))
	for agentID, pairCount := range agentPairs {
		linkedAgents = append(linkedAgents, domain.LinkedAgent{SupportAgentID: agentID, PairCount: pairCount})
	}
	sort.Slice(linkedAgents, func(i, j int) bool {
		return linkedAgents[i].PairCount > linkedAgents[j].PairCount
	})

	return &domain.CustomerHistory{
		CustomerID:      customerID,
		OrdersCount:     len(customerRecords),
		ReturnCount:     len(customerRecords),
		ApprovedRefunds: approvedCount,
		RecentReturns:   recentReturns,
		LinkedAgents:    linkedAgents,
	}
}

func (s *Service) GetAgentSummary(agentID string) *domain.AgentSummary {
	agentRecords := make([]domain.NormalizedReturnRecord, 0)
	customerPairs := map[string]int{}
	categoryCounts := map[string]int{}
	approvedCount := 0
	manualOverrideCount := 0
	highValueApprovalCount := 0

	for _, record := range s.records {
		if record.SupportAgentID != agentID {
			continue
		}
		agentRecords = append(agentRecords, record)
		customerPairs[record.CustomerID]++
		categoryCounts[record.ProductCategory]++
		if record.DecisionStatus == approvedDecision {
			approvedCount++
			if record.RefundAmount >= 300 {
				highValueApprovalCount++
			}
		}
		if record.ManualOverride {
			manualOverrideCount++
		}
	}

	repeatedCustomerPairCount := 0
	for _, count := range customerPairs {
		if count > repeatedCustomerPairCount {
			repeatedCustomerPairCount = count
		}
	}

	return &domain.AgentSummary{
		SupportAgentID:            agentID,
		DecisionsCount:            len(agentRecords),
		ApprovalRate:              ratio(approvedCount, len(agentRecords)),
		HighValueApprovalCount:    highValueApprovalCount,
		ManualOverrideCount:       manualOverrideCount,
		RepeatedCustomerPairCount: repeatedCustomerPairCount,
		TopRiskyCategory:          topKey(categoryCounts),
	}
}

func (s *Service) GetReturnFeatures(returnID string) (*domain.ReturnFeaturesResponse, error) {
	record, ok := s.findRecord(returnID)
	if !ok {
		return nil, ErrReturnNotFound
	}

	features := s.calculateFeatures(record)
	return &domain.ReturnFeaturesResponse{
		ReturnID:       record.ReturnID,
		CustomerID:     record.CustomerID,
		SupportAgentID: record.SupportAgentID,
		Features:       features,
	}, nil
}

func (s *Service) GetReturnFeaturesForDataset(datasetID, returnID string) (*domain.ReturnFeaturesResponse, error) {
	record, ok := s.findRecordInDataset(datasetID, returnID)
	if !ok {
		return nil, ErrReturnNotFound
	}

	features := s.calculateFeatures(record)
	return &domain.ReturnFeaturesResponse{
		ReturnID:       record.ReturnID,
		CustomerID:     record.CustomerID,
		SupportAgentID: record.SupportAgentID,
		Features:       features,
	}, nil
}

func (s *Service) calculateFeatures(target domain.NormalizedReturnRecord) domain.RelationFeatures {
	customerReturnCount := 0
	customerApprovedRefundCount := 0
	agentDecisionCount := 0
	agentApprovedCount := 0
	agentManualOverrideCount := 0
	agentHighValueApprovalCount := 0
	customerAgentPairCount := 0
	categoryReturnCount := 0
	sameReasonRefundCount := 0
	similarReturnsCount := 0
	topRelatedReturns := make([]string, 0)

	for _, record := range s.records {
		if record.CustomerID == target.CustomerID {
			customerReturnCount++
			if record.DecisionStatus == approvedDecision {
				customerApprovedRefundCount++
			}
		}

		if record.SupportAgentID == target.SupportAgentID {
			agentDecisionCount++
			if record.DecisionStatus == approvedDecision {
				agentApprovedCount++
				if record.RefundAmount >= 300 {
					agentHighValueApprovalCount++
				}
			}
			if record.ManualOverride {
				agentManualOverrideCount++
			}
		}

		if record.CustomerID == target.CustomerID && record.SupportAgentID == target.SupportAgentID {
			customerAgentPairCount++
			if record.ReturnID != target.ReturnID {
				topRelatedReturns = append(topRelatedReturns, record.ReturnID)
			}
		}

		if record.ProductCategory == target.ProductCategory {
			categoryReturnCount++
		}

		if record.ReturnReason == target.ReturnReason && record.SupportAgentID == target.SupportAgentID {
			sameReasonRefundCount++
		}

		if record.ReturnID != target.ReturnID &&
			record.ReturnReason == target.ReturnReason &&
			record.ProductCategory == target.ProductCategory &&
			record.SupportAgentID == target.SupportAgentID {
			similarReturnsCount++
			topRelatedReturns = append(topRelatedReturns, record.ReturnID)
		}
	}

	topRelatedReturns = uniqueStrings(topRelatedReturns)
	clusterSize := maxInt(customerReturnCount, agentDecisionCount, customerAgentPairCount, sameReasonRefundCount)

	strongestRelation := strongestRelationType(customerReturnCount, agentDecisionCount, customerAgentPairCount, sameReasonRefundCount)
	signals := explanationSignals(
		customerReturnCount,
		agentApprovedCount,
		agentDecisionCount,
		customerAgentPairCount,
		sameReasonRefundCount,
		clusterSize,
		ratioFloat(target.RefundAmount, target.OrderAmount),
		topRelatedReturns,
	)

	return domain.RelationFeatures{
		CustomerReturnCount:           customerReturnCount,
		CustomerApprovedRefundCount:   customerApprovedRefundCount,
		AgentApprovalRate:             ratio(agentApprovedCount, agentDecisionCount),
		AgentManualOverrideRate:       ratio(agentManualOverrideCount, agentDecisionCount),
		AgentHighValueApprovalCount:   agentHighValueApprovalCount,
		CustomerAgentPairCount:        customerAgentPairCount,
		AgentCustomerInteractionCount: customerAgentPairCount,
		CategoryRefundRate:            ratio(categoryReturnCount, len(s.records)),
		RefundAmountRatio:             ratioFloat(target.RefundAmount, target.OrderAmount),
		SimilarReturnsCount:           similarReturnsCount,
		SameReasonRefundCount:         sameReasonRefundCount,
		ClusterSize:                   clusterSize,
		StrongestRelationType:         strongestRelation,
		TopRelatedReturns:             topRelatedReturns,
		ExplanationSummary:            explanationSummary(strongestRelation),
		ExplanationSignals:            signals,
	}
}

type buildStats struct {
	RelationsCount int
	FeaturesCount  int
}

func (s *Service) buildStats(datasetID string) buildStats {
	featuresCount := 0
	relationsCount := 0
	for _, record := range s.records {
		if record.DatasetID != datasetID {
			continue
		}
		featuresCount++
		relationsCount += 7
	}

	return buildStats{
		RelationsCount: relationsCount,
		FeaturesCount:  featuresCount,
	}
}

func (s *Service) publishRelationsBuilt(ctx context.Context, datasetID, jobID string, stats buildStats) error {
	if s.publisher == nil {
		return nil
	}

	return s.publisher.PublishRelationsBuilt(ctx, RelationsBuiltEvent{
		DatasetID:      datasetID,
		JobID:          jobID,
		RelationsCount: stats.RelationsCount,
		FeaturesCount:  stats.FeaturesCount,
		PublishedAt:    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Service) findRecord(returnID string) (domain.NormalizedReturnRecord, bool) {
	for _, record := range s.records {
		if record.ReturnID == returnID {
			return record, true
		}
	}
	return domain.NormalizedReturnRecord{}, false
}

func (s *Service) findRecordInDataset(datasetID, returnID string) (domain.NormalizedReturnRecord, bool) {
	for _, record := range s.records {
		if record.DatasetID == datasetID && record.ReturnID == returnID {
			return record, true
		}
	}
	return domain.NormalizedReturnRecord{}, false
}

func loadRecords(options Options) ([]domain.NormalizedReturnRecord, error) {
	datasetID := options.DatasetID
	if datasetID == "" {
		datasetID = "demo"
	}

	for _, path := range candidateDatasetPaths(options.DatasetPath) {
		records, err := loadRecordsFromCSV(datasetID, path)
		if err == nil {
			return records, nil
		}
	}

	return nil, errors.New("normalized refund dataset was not found")
}

func candidateDatasetPaths(configuredPath string) []string {
	paths := make([]string, 0, 4)
	if configuredPath != "" {
		paths = append(paths, configuredPath)
	}

	paths = append(paths,
		"/data/clean_refund_dataset.csv",
		filepath.Clean("../../data/clean_refund_dataset.csv"),
		filepath.Clean("data/clean_refund_dataset.csv"),
	)

	return paths
}

func loadRecordsFromCSV(datasetID, path string) ([]domain.NormalizedReturnRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	index := map[string]int{}
	for i, name := range header {
		index[name] = i
	}

	records := make([]domain.NormalizedReturnRecord, 0)
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		record, err := parseNormalizedReturnRecord(datasetID, row, index)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, errors.New("normalized refund dataset is empty")
	}

	return records, nil
}

func parseNormalizedReturnRecord(datasetID string, row []string, index map[string]int) (domain.NormalizedReturnRecord, error) {
	orderAmount, err := parseFloat(row, index, "order_amount")
	if err != nil {
		return domain.NormalizedReturnRecord{}, err
	}
	refundAmount, err := parseFloat(row, index, "refund_amount")
	if err != nil {
		return domain.NormalizedReturnRecord{}, err
	}
	manualOverride, err := parseBool(row, index, "manual_override")
	if err != nil {
		return domain.NormalizedReturnRecord{}, err
	}
	decisionTimeMinutes, err := parseInt(row, index, "decision_time_minutes")
	if err != nil {
		return domain.NormalizedReturnRecord{}, err
	}

	return domain.NormalizedReturnRecord{
		DatasetID:       datasetID,
		ReturnID:        readString(row, index, "return_id"),
		CustomerID:      readString(row, index, "customer_id"),
		OrderID:         readString(row, index, "order_id"),
		SupportAgentID:  readString(row, index, "support_agent_id"),
		ProductCategory: readString(row, index, "product_category"),
		ReturnReason:    readString(row, index, "return_reason"),
		DecisionID:      fmt.Sprintf("decision_%s", strings.TrimPrefix(readString(row, index, "return_id"), "return_")),
		DecisionStatus:  strings.ToUpper(readString(row, index, "decision")),
		RefundAmount:    refundAmount,
		OrderAmount:     orderAmount,
		ManualOverride:  manualOverride,
		DecisionTimeMs:  decisionTimeMinutes * 60 * 1000,
	}, nil
}

func readString(row []string, index map[string]int, column string) string {
	i, ok := index[column]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func parseFloat(row []string, index map[string]int, column string) (float64, error) {
	value := readString(row, index, column)
	return strconv.ParseFloat(value, 64)
}

func parseBool(row []string, index map[string]int, column string) (bool, error) {
	value := readString(row, index, column)
	return strconv.ParseBool(value)
}

func parseInt(row []string, index map[string]int, column string) (int, error) {
	value := readString(row, index, column)
	return strconv.Atoi(value)
}

func fallbackTestDataset() []domain.NormalizedReturnRecord {
	return []domain.NormalizedReturnRecord{
		{
			DatasetID:       "demo",
			ReturnID:        "return_3041",
			CustomerID:      "customer_880",
			OrderID:         "order_9101",
			SupportAgentID:  "agent_017",
			ProductCategory: "electronics",
			ReturnReason:    "item_not_as_described",
			DecisionID:      "decision_7001",
			DecisionStatus:  approvedDecision,
			RefundAmount:    420.00,
			OrderAmount:     520.00,
			ManualOverride:  true,
			DecisionTimeMs:  3900,
		},
		{
			DatasetID:       "demo",
			ReturnID:        "return_3006",
			CustomerID:      "customer_880",
			OrderID:         "order_9066",
			SupportAgentID:  "agent_017",
			ProductCategory: "electronics",
			ReturnReason:    "item_not_as_described",
			DecisionID:      "decision_6966",
			DecisionStatus:  approvedDecision,
			RefundAmount:    310.00,
			OrderAmount:     340.00,
			ManualOverride:  false,
			DecisionTimeMs:  4400,
		},
		{
			DatasetID:       "demo",
			ReturnID:        "return_3022",
			CustomerID:      "customer_880",
			OrderID:         "order_9082",
			SupportAgentID:  "agent_019",
			ProductCategory: "apparel",
			ReturnReason:    "wrong_size",
			DecisionID:      "decision_6982",
			DecisionStatus:  "REJECTED",
			RefundAmount:    0,
			OrderAmount:     95.50,
			ManualOverride:  false,
			DecisionTimeMs:  6200,
		},
		{
			DatasetID:       "demo",
			ReturnID:        "return_3110",
			CustomerID:      "customer_884",
			OrderID:         "order_9170",
			SupportAgentID:  "agent_017",
			ProductCategory: "electronics",
			ReturnReason:    "item_not_as_described",
			DecisionID:      "decision_7070",
			DecisionStatus:  approvedDecision,
			RefundAmount:    280.00,
			OrderAmount:     300.00,
			ManualOverride:  true,
			DecisionTimeMs:  5100,
		},
		{
			DatasetID:       "demo",
			ReturnID:        "return_3188",
			CustomerID:      "customer_901",
			OrderID:         "order_9248",
			SupportAgentID:  "agent_017",
			ProductCategory: "electronics",
			ReturnReason:    "damaged_item",
			DecisionID:      "decision_7148",
			DecisionStatus:  "REJECTED",
			RefundAmount:    0,
			OrderAmount:     610.00,
			ManualOverride:  false,
			DecisionTimeMs:  7300,
		},
	}
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return round(float64(numerator) / float64(denominator))
}

func ratioFloat(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return round(numerator / denominator)
}

func round(value float64) float64 {
	return math.Round(value*100) / 100
}

func maxInt(values ...int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func topKey(counts map[string]int) string {
	top := ""
	topCount := 0
	for key, count := range counts {
		if count > topCount || top == "" {
			top = key
			topCount = count
		}
	}
	return top
}

func strongestRelationType(customerReturnCount, agentDecisionCount, customerAgentPairCount, sameReasonRefundCount int) string {
	type candidate struct {
		name  string
		count int
	}

	candidates := []candidate{
		{name: "CUSTOMER_RETURN_PATTERN", count: customerReturnCount},
		{name: "AGENT_DECISION_PATTERN", count: agentDecisionCount},
		{name: "CUSTOMER_AGENT_PAIR", count: customerAgentPairCount},
		{name: "SAME_REASON_PATTERN", count: sameReasonRefundCount},
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].count > candidates[j].count
	})

	return candidates[0].name
}

func explanationSummary(strongestRelation string) string {
	switch strongestRelation {
	case "CUSTOMER_RETURN_PATTERN":
		return "Customer refund history is the strongest relation signal."
	case "AGENT_DECISION_PATTERN":
		return "Support agent decision history is the strongest relation signal."
	case "CUSTOMER_AGENT_PAIR":
		return "Repeated customer-agent interaction is the strongest relation signal."
	case "SAME_REASON_PATTERN":
		return "Shared return reason is the strongest relation signal."
	default:
		return "Relation features provide additional investigation context."
	}
}

func explanationSignals(customerReturnCount, agentApprovedCount, agentDecisionCount, customerAgentPairCount, sameReasonRefundCount, clusterSize int, refundAmountRatio float64, topRelatedReturns []string) []string {
	signals := make([]string, 0, 6)

	if customerReturnCount >= 5 {
		signals = append(signals, fmt.Sprintf("Customer has %d refund requests in the demo dataset.", customerReturnCount))
	}
	if agentDecisionCount > 0 && ratio(agentApprovedCount, agentDecisionCount) >= 0.9 {
		signals = append(signals, fmt.Sprintf("Support agent approval rate is %.0f%%.", ratio(agentApprovedCount, agentDecisionCount)*100))
	}
	if customerAgentPairCount >= 3 {
		signals = append(signals, fmt.Sprintf("Customer and support agent interacted on %d return requests.", customerAgentPairCount))
	}
	if sameReasonRefundCount >= 5 {
		signals = append(signals, fmt.Sprintf("%d returns share the same return reason.", sameReasonRefundCount))
	}
	if refundAmountRatio >= 0.8 {
		signals = append(signals, fmt.Sprintf("Refund amount is %.0f%% of the original order amount.", refundAmountRatio*100))
	}
	if clusterSize >= 5 {
		signals = append(signals, fmt.Sprintf("Relation cluster fallback size is %d.", clusterSize))
	}
	if len(topRelatedReturns) > 0 {
		signals = append(signals, fmt.Sprintf("Related returns for investigation: %s.", strings.Join(topRelatedReturns, ", ")))
	}

	if len(signals) == 0 {
		signals = append(signals, "No strong relation signal crossed the current demo thresholds.")
	}

	return signals
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}
