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
	"sync"
	"time"

	"relations-service/internal/domain"
)

const (
	approvedDecision      = "APPROVED"
	defaultDatasetID      = "demo"
	defaultSchemaVersion  = "refund-normalized.v1"
	defaultRelatedLimit   = 8
	defaultAnalyticsLimit = 10
	defaultGraphLimit     = 24
	highValueRefundAmount = 300.0
)

var (
	ErrDatasetNotFound = errors.New("dataset not found")
	ErrReturnNotFound  = errors.New("return not found")
	ErrInvalidDataset  = errors.New("invalid normalized dataset")
)

type RelationsBuiltPublisher interface {
	PublishRelationsBuilt(ctx context.Context, event RelationsBuiltEvent) error
	PublishPipelineFailed(ctx context.Context, event PipelineFailedEvent) error
}

type Service struct {
	mu                sync.RWMutex
	publisher         RelationsBuiltPublisher
	datasets          map[string]*datasetState
	allowDemoFallback bool
}

type datasetState struct {
	metadata  domain.DatasetMetadata
	records   []domain.NormalizedReturnRecord
	features  map[string]domain.ReturnFeaturesResponse
	relations int
}

type Options struct {
	DatasetID         string
	DatasetPath       string
	AllowDemoFallback bool
}

type NormalizedDatasetEvent struct {
	DatasetID     string `json:"datasetId"`
	JobID         string `json:"jobId"`
	RecordsPath   string `json:"recordsPath,omitempty"`
	RecordCount   int    `json:"recordCount,omitempty"`
	SchemaVersion string `json:"schemaVersion,omitempty"`
	PublishedAt   string `json:"publishedAt,omitempty"`
}

type RelationsBuiltEvent struct {
	DatasetID      string `json:"datasetId"`
	JobID          string `json:"jobId"`
	RecordsPath    string `json:"recordsPath,omitempty"`
	RecordsCount   int    `json:"recordsCount"`
	RelationsCount int    `json:"relationsCount"`
	FeaturesCount  int    `json:"featuresCount"`
	SchemaVersion  string `json:"schemaVersion"`
	FeatureVersion int64  `json:"featureVersion"`
	PublishedAt    string `json:"publishedAt"`
}

type PipelineFailedEvent struct {
	DatasetID string `json:"datasetId"`
	JobID     string `json:"jobId"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	FailedAt  string `json:"failedAt"`
}

func NewService(publisher RelationsBuiltPublisher, options Options) *Service {
	s := &Service{
		publisher:         publisher,
		datasets:          map[string]*datasetState{},
		allowDemoFallback: options.AllowDemoFallback,
	}

	datasetID := options.DatasetID
	if datasetID == "" {
		datasetID = defaultDatasetID
	}

	if options.DatasetPath != "" {
		_, _ = s.loadDatasetFromPath(datasetID, "", options.DatasetPath, defaultSchemaVersion, 0)
		return s
	}

	if options.AllowDemoFallback {
		_, _ = s.replaceDataset(datasetID, "", "embedded-demo-fallback", defaultSchemaVersion, fallbackTestDataset())
	}

	return s
}

func NewServiceWithRecords(publisher RelationsBuiltPublisher, records []domain.NormalizedReturnRecord) *Service {
	s := &Service{
		publisher:         publisher,
		datasets:          map[string]*datasetState{},
		allowDemoFallback: true,
	}
	datasetID := defaultDatasetID
	if len(records) > 0 && records[0].DatasetID != "" {
		datasetID = records[0].DatasetID
	}
	_, _ = s.replaceDataset(datasetID, "", "test-records", defaultSchemaVersion, records)
	return s
}

func (s *Service) Health() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"status":        "UP",
		"service":       "relations-service",
		"datasetsCount": len(s.datasets),
	}
}

func (s *Service) RebuildDataset(ctx context.Context, datasetID string) (*domain.DatasetRebuildResponse, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}

	if err := s.publishRelationsBuilt(ctx, datasetID, fmt.Sprintf("relations-job-%s", datasetID), state); err != nil {
		return nil, err
	}

	return rebuildResponse(state, fmt.Sprintf("relations-job-%s", datasetID), "RELATIONS_REBUILD_COMPLETED"), nil
}

func (s *Service) ProcessNormalizedDataset(ctx context.Context, event NormalizedDatasetEvent) error {
	if event.DatasetID == "" {
		err := fmt.Errorf("%w: datasetId is required", ErrInvalidDataset)
		_ = s.publishPipelineFailed(ctx, event, err)
		return err
	}
	if event.RecordsPath == "" {
		err := fmt.Errorf("%w: recordsPath is required for dataset %s", ErrInvalidDataset, event.DatasetID)
		_ = s.publishPipelineFailed(ctx, event, err)
		return err
	}

	schemaVersion := event.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = defaultSchemaVersion
	}

	state, err := s.loadDatasetFromPath(event.DatasetID, event.JobID, event.RecordsPath, schemaVersion, event.RecordCount)
	if err != nil {
		_ = s.publishPipelineFailed(ctx, event, err)
		return err
	}

	return s.publishRelationsBuilt(ctx, event.DatasetID, event.JobID, state)
}

func (s *Service) GetReturnRelations(returnID string) (*domain.ReturnRelations, error) {
	return s.GetReturnRelationsForDataset(defaultDatasetID, returnID)
}

func (s *Service) GetReturnRelationsForDataset(datasetID, returnID string) (*domain.ReturnRelations, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}

	record, ok := findRecord(state.records, returnID)
	if !ok {
		return nil, ErrReturnNotFound
	}

	return buildReturnRelations(record), nil
}

func (s *Service) GetCustomerHistory(customerID string) *domain.CustomerHistory {
	history, _ := s.GetCustomerHistoryForDataset(defaultDatasetID, customerID)
	if history == nil {
		return &domain.CustomerHistory{CustomerID: customerID}
	}
	return history
}

func (s *Service) GetCustomerHistoryForDataset(datasetID, customerID string) (*domain.CustomerHistory, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	return buildCustomerHistory(customerID, state.records), nil
}

func (s *Service) GetCustomerBehaviorSummary(datasetID, customerID string, limit int) (*domain.CustomerBehaviorSummary, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultAnalyticsLimit
	}

	history := buildCustomerHistory(customerID, state.records)
	if len(history.RecentReturns) > limit {
		history.RecentReturns = history.RecentReturns[:limit]
	}

	totalRefund := 0.0
	totalRatio := 0.0
	count := 0
	for _, record := range state.records {
		if record.CustomerID != customerID {
			continue
		}
		totalRefund += record.RefundAmount
		totalRatio += ratioFloat(record.RefundAmount, record.OrderAmount)
		count++
	}

	return &domain.CustomerBehaviorSummary{
		DatasetID:           datasetID,
		CustomerID:          customerID,
		ReturnCount:         history.ReturnCount,
		ApprovedRefundCount: history.ApprovedRefunds,
		TotalRefundAmount:   round(totalRefund),
		AverageRefundRatio:  ratioFloat(totalRatio, float64(count)),
		RelatedAgents:       history.LinkedAgents,
		RecentReturns:       history.RecentReturns,
	}, nil
}

func (s *Service) GetAgentSummary(agentID string) *domain.AgentSummary {
	summary, _ := s.GetAgentSummaryForDataset(defaultDatasetID, agentID)
	if summary == nil {
		return &domain.AgentSummary{SupportAgentID: agentID}
	}
	return summary
}

func (s *Service) GetAgentSummaryForDataset(datasetID, agentID string) (*domain.AgentSummary, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	summary := buildAgentSummary(agentID, state.records, state.features)
	return &domain.AgentSummary{
		SupportAgentID:            summary.SupportAgentID,
		DecisionsCount:            summary.DecisionsCount,
		ApprovalRate:              summary.ApprovalRate,
		HighValueApprovalCount:    summary.HighValueApprovalCount,
		ManualOverrideCount:       int(round(summary.ManualOverrideRate * float64(summary.DecisionsCount))),
		RepeatedCustomerPairCount: summary.RepeatedCustomerPairCount,
		TopRiskyCategory:          summary.TopRiskyCategory,
	}, nil
}

func (s *Service) GetRankedAgents(datasetID string, limit int, sortBy string) (*domain.RankedAgentsResponse, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultAnalyticsLimit
	}
	if sortBy == "" {
		sortBy = "averageClusterSize"
	}

	agentIDs := map[string]struct{}{}
	for _, record := range state.records {
		agentIDs[record.SupportAgentID] = struct{}{}
	}

	agents := make([]domain.RankedAgentSummary, 0, len(agentIDs))
	for agentID := range agentIDs {
		agents = append(agents, buildAgentSummary(agentID, state.records, state.features))
	}

	sort.SliceStable(agents, func(i, j int) bool {
		switch sortBy {
		case "approvalRate":
			if agents[i].ApprovalRate == agents[j].ApprovalRate {
				return agents[i].SupportAgentID < agents[j].SupportAgentID
			}
			return agents[i].ApprovalRate > agents[j].ApprovalRate
		case "manualOverrideRate":
			if agents[i].ManualOverrideRate == agents[j].ManualOverrideRate {
				return agents[i].SupportAgentID < agents[j].SupportAgentID
			}
			return agents[i].ManualOverrideRate > agents[j].ManualOverrideRate
		default:
			if agents[i].AverageClusterSize == agents[j].AverageClusterSize {
				return agents[i].SupportAgentID < agents[j].SupportAgentID
			}
			return agents[i].AverageClusterSize > agents[j].AverageClusterSize
		}
	})

	if len(agents) > limit {
		agents = agents[:limit]
	}

	return &domain.RankedAgentsResponse{DatasetID: datasetID, Agents: agents, Limit: limit, Sort: sortBy}, nil
}

func (s *Service) GetReturnFeatures(returnID string) (*domain.ReturnFeaturesResponse, error) {
	return s.GetReturnFeaturesForDataset(defaultDatasetID, returnID)
}

func (s *Service) GetReturnFeaturesForDataset(datasetID, returnID string) (*domain.ReturnFeaturesResponse, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	features, ok := state.features[returnID]
	if !ok {
		return nil, ErrReturnNotFound
	}
	return &features, nil
}

func (s *Service) GetRelatedReturns(datasetID, returnID string, limit int) (*domain.RelatedReturnsResponse, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	target, ok := findRecord(state.records, returnID)
	if !ok {
		return nil, ErrReturnNotFound
	}
	if limit <= 0 {
		limit = defaultRelatedLimit
	}

	related := relatedReturns(target, state.records, limit)
	return &domain.RelatedReturnsResponse{
		DatasetID:      datasetID,
		ReturnID:       returnID,
		RelatedReturns: related.Items,
		Limit:          limit,
		Truncated:      related.Truncated,
	}, nil
}

func (s *Service) GetGraphProjection(datasetID, returnID string, limit int) (*domain.GraphProjectionResponse, error) {
	state, err := s.requireDataset(datasetID)
	if err != nil {
		return nil, err
	}
	target, ok := findRecord(state.records, returnID)
	if !ok {
		return nil, ErrReturnNotFound
	}
	if limit <= 0 {
		limit = defaultGraphLimit
	}

	features, ok := state.features[returnID]
	if !ok {
		return nil, ErrReturnNotFound
	}
	related := relatedReturns(target, state.records, maxInt(1, limit-6))

	nodes := []domain.GraphNode{
		graphNode("return:"+target.ReturnID, "return", target.ReturnID, "Selected return request", map[string]interface{}{"refundAmount": target.RefundAmount, "refundAmountRatio": features.Features.RefundAmountRatio}),
		graphNode("customer:"+target.CustomerID, "customer", target.CustomerID, "Customer who requested the refund", map[string]interface{}{"customerReturnCount": features.Features.CustomerReturnCount}),
		graphNode("order:"+target.OrderID, "order", target.OrderID, "Original order", map[string]interface{}{"orderAmount": target.OrderAmount}),
		graphNode("agent:"+target.SupportAgentID, "supportAgent", target.SupportAgentID, "Support agent who handled the decision", map[string]interface{}{"agentApprovalRate": features.Features.AgentApprovalRate}),
		graphNode("decision:"+target.DecisionID, "decision", target.DecisionStatus, "Support decision", map[string]interface{}{"manualOverride": target.ManualOverride}),
		graphNode("category:"+target.ProductCategory, "productCategory", target.ProductCategory, "Product category", map[string]interface{}{"categoryRefundRate": features.Features.CategoryRefundRate}),
	}
	edges := []domain.GraphEdge{
		graphEdge("customer:"+target.CustomerID, "order:"+target.OrderID, "PLACED_ORDER", "placed order", 1, 1, "Customer placed the original order."),
		graphEdge("order:"+target.OrderID, "return:"+target.ReturnID, "HAS_RETURN_REQUEST", "has return request", 1, 1, "Order has the selected return request."),
		graphEdge("return:"+target.ReturnID, "agent:"+target.SupportAgentID, "DECIDED_BY", "decided by", 1, features.Features.AgentApprovalRate, "Return was handled by this support agent."),
		graphEdge("agent:"+target.SupportAgentID, "decision:"+target.DecisionID, "MADE_DECISION", "made decision", 1, 1, "Agent made the approval/rejection decision."),
		graphEdge("order:"+target.OrderID, "category:"+target.ProductCategory, "HAS_CATEGORY", "has category", 1, features.Features.CategoryRefundRate, "Order belongs to this product category."),
	}

	for _, item := range related.Items {
		nodeID := "return:" + item.ReturnID
		nodes = append(nodes, graphNode(nodeID, "relatedReturn", item.ReturnID, item.Reason, map[string]interface{}{"strength": item.Strength, "relationType": item.RelationType}))
		edges = append(edges, graphEdge("return:"+target.ReturnID, nodeID, item.RelationType, relationLabel(item.RelationType), itemCount(item), item.Strength, item.Reason))
	}

	nodes = dedupeNodes(nodes)
	edges = dedupeEdges(edges)
	truncated := related.Truncated || len(nodes) > limit
	if len(nodes) > limit {
		nodes = nodes[:limit]
	}

	return &domain.GraphProjectionResponse{
		DatasetID: datasetID,
		ReturnID:  returnID,
		Nodes:     nodes,
		Edges:     edges,
		Limit:     limit,
		Truncated: truncated,
	}, nil
}

func (s *Service) loadDatasetFromPath(datasetID, jobID, source, schemaVersion string, expectedCount int) (*datasetState, error) {
	records, err := loadRecordsFromCSV(datasetID, source)
	if err != nil {
		return nil, err
	}
	if expectedCount > 0 && len(records) != expectedCount {
		return nil, fmt.Errorf("%w: recordCount mismatch for dataset %s: got %d want %d", ErrInvalidDataset, datasetID, len(records), expectedCount)
	}
	return s.replaceDataset(datasetID, jobID, source, schemaVersion, records)
}

func (s *Service) replaceDataset(datasetID, jobID, source, schemaVersion string, records []domain.NormalizedReturnRecord) (*datasetState, error) {
	if datasetID == "" {
		return nil, fmt.Errorf("%w: datasetId is required", ErrInvalidDataset)
	}
	if schemaVersion == "" {
		schemaVersion = defaultSchemaVersion
	}
	if err := validateRecords(datasetID, records); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	version := now.UnixNano()
	features := buildFeatureCache(datasetID, version, now, records)
	state := &datasetState{
		metadata: domain.DatasetMetadata{
			DatasetID:      datasetID,
			RecordsCount:   len(records),
			SchemaVersion:  schemaVersion,
			Source:         source,
			FeatureVersion: version,
			LoadedAt:       now.Format(time.RFC3339),
			CalculatedAt:   now.Format(time.RFC3339),
		},
		records:   records,
		features:  features,
		relations: len(records) * 7,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.datasets[datasetID] = state
	return state, nil
}

func (s *Service) requireDataset(datasetID string) (*datasetState, error) {
	if datasetID == "" {
		datasetID = defaultDatasetID
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.datasets[datasetID]
	if !ok {
		return nil, ErrDatasetNotFound
	}
	return cloneDatasetState(state), nil
}

func cloneDatasetState(state *datasetState) *datasetState {
	records := append([]domain.NormalizedReturnRecord(nil), state.records...)
	features := make(map[string]domain.ReturnFeaturesResponse, len(state.features))
	for key, value := range state.features {
		features[key] = value
	}
	return &datasetState{
		metadata:  state.metadata,
		records:   records,
		features:  features,
		relations: state.relations,
	}
}

func buildFeatureCache(datasetID string, version int64, calculatedAt time.Time, records []domain.NormalizedReturnRecord) map[string]domain.ReturnFeaturesResponse {
	features := make(map[string]domain.ReturnFeaturesResponse, len(records))
	for _, record := range records {
		relationFeatures := calculateFeatures(record, records)
		features[record.ReturnID] = domain.ReturnFeaturesResponse{
			ReturnID:       record.ReturnID,
			DatasetID:      datasetID,
			CustomerID:     record.CustomerID,
			SupportAgentID: record.SupportAgentID,
			FeatureVersion: version,
			CalculatedAt:   calculatedAt.Format(time.RFC3339),
			Features:       relationFeatures,
		}
	}
	return features
}

func calculateFeatures(target domain.NormalizedReturnRecord, records []domain.NormalizedReturnRecord) domain.RelationFeatures {
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

	for _, record := range records {
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
				if record.RefundAmount >= highValueRefundAmount {
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

	topRelatedReturns = limitStrings(uniqueStrings(topRelatedReturns), defaultRelatedLimit)
	clusterSize := maxInt(customerReturnCount, agentDecisionCount, customerAgentPairCount, sameReasonRefundCount)
	strongestRelation := strongestRelationType(customerReturnCount, agentDecisionCount, customerAgentPairCount, sameReasonRefundCount)
	refundAmountRatio := ratioFloat(target.RefundAmount, target.OrderAmount)

	return domain.RelationFeatures{
		CustomerReturnCount:           customerReturnCount,
		CustomerApprovedRefundCount:   customerApprovedRefundCount,
		AgentApprovalRate:             ratio(agentApprovedCount, agentDecisionCount),
		AgentManualOverrideRate:       ratio(agentManualOverrideCount, agentDecisionCount),
		AgentHighValueApprovalCount:   agentHighValueApprovalCount,
		CustomerAgentPairCount:        customerAgentPairCount,
		AgentCustomerInteractionCount: customerAgentPairCount,
		CategoryRefundRate:            ratio(categoryReturnCount, len(records)),
		RefundAmountRatio:             refundAmountRatio,
		SimilarReturnsCount:           similarReturnsCount,
		SameReasonRefundCount:         sameReasonRefundCount,
		ClusterSize:                   clusterSize,
		StrongestRelationType:         strongestRelation,
		TopRelatedReturns:             topRelatedReturns,
		ExplanationSummary:            explanationSummary(strongestRelation),
		ExplanationSignals: explanationSignals(
			customerReturnCount,
			agentApprovedCount,
			agentDecisionCount,
			customerAgentPairCount,
			sameReasonRefundCount,
			clusterSize,
			refundAmountRatio,
			topRelatedReturns,
		),
	}
}

func buildReturnRelations(record domain.NormalizedReturnRecord) *domain.ReturnRelations {
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
			DecisionID:          record.DecisionID,
			Status:              record.DecisionStatus,
			RefundAmount:        record.RefundAmount,
			ManualOverride:      record.ManualOverride,
			DecisionTimeMinutes: record.DecisionTimeMinutes,
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
	}
}

func buildCustomerHistory(customerID string, records []domain.NormalizedReturnRecord) *domain.CustomerHistory {
	customerRecords := make([]domain.NormalizedReturnRecord, 0)
	agentPairs := map[string]int{}
	approvedCount := 0

	for _, record := range records {
		if record.CustomerID != customerID {
			continue
		}
		customerRecords = append(customerRecords, record)
		agentPairs[record.SupportAgentID]++
		if record.DecisionStatus == approvedDecision {
			approvedCount++
		}
	}

	sort.Slice(customerRecords, func(i, j int) bool {
		return customerRecords[i].ReturnID > customerRecords[j].ReturnID
	})

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
	sort.SliceStable(linkedAgents, func(i, j int) bool {
		if linkedAgents[i].PairCount == linkedAgents[j].PairCount {
			return linkedAgents[i].SupportAgentID < linkedAgents[j].SupportAgentID
		}
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

func buildAgentSummary(agentID string, records []domain.NormalizedReturnRecord, features map[string]domain.ReturnFeaturesResponse) domain.RankedAgentSummary {
	agentRecords := make([]domain.NormalizedReturnRecord, 0)
	customerPairs := map[string]int{}
	categoryCounts := map[string]int{}
	approvedCount := 0
	manualOverrideCount := 0
	highValueApprovalCount := 0
	totalClusterSize := 0

	for _, record := range records {
		if record.SupportAgentID != agentID {
			continue
		}
		agentRecords = append(agentRecords, record)
		customerPairs[record.CustomerID]++
		categoryCounts[record.ProductCategory]++
		if record.DecisionStatus == approvedDecision {
			approvedCount++
			if record.RefundAmount >= highValueRefundAmount {
				highValueApprovalCount++
			}
		}
		if record.ManualOverride {
			manualOverrideCount++
		}
		if cached, ok := features[record.ReturnID]; ok {
			totalClusterSize += cached.Features.ClusterSize
		}
	}

	repeatedCustomerPairCount := 0
	for _, count := range customerPairs {
		if count > repeatedCustomerPairCount {
			repeatedCustomerPairCount = count
		}
	}

	return domain.RankedAgentSummary{
		SupportAgentID:            agentID,
		DecisionsCount:            len(agentRecords),
		ApprovalRate:              ratio(approvedCount, len(agentRecords)),
		ManualOverrideRate:        ratio(manualOverrideCount, len(agentRecords)),
		HighValueApprovalCount:    highValueApprovalCount,
		RepeatedCustomerPairCount: repeatedCustomerPairCount,
		AverageClusterSize:        ratioFloat(float64(totalClusterSize), float64(len(agentRecords))),
		TopRiskyCategory:          topKey(categoryCounts),
	}
}

type relatedResult struct {
	Items     []domain.RelatedReturn
	Truncated bool
}

func relatedReturns(target domain.NormalizedReturnRecord, records []domain.NormalizedReturnRecord, limit int) relatedResult {
	items := make([]domain.RelatedReturn, 0)
	for _, record := range records {
		if record.ReturnID == target.ReturnID {
			continue
		}

		relationType := ""
		reason := ""
		strength := 0.0
		switch {
		case record.CustomerID == target.CustomerID && record.SupportAgentID == target.SupportAgentID:
			relationType = "CUSTOMER_AGENT_PAIR"
			reason = "Same customer and support agent pair."
			strength = 1.0
		case record.CustomerID == target.CustomerID:
			relationType = "SAME_CUSTOMER"
			reason = "Same customer refund history."
			strength = 0.8
		case record.SupportAgentID == target.SupportAgentID && record.ReturnReason == target.ReturnReason && record.ProductCategory == target.ProductCategory:
			relationType = "SIMILAR_AGENT_REASON_CATEGORY"
			reason = "Same agent, return reason, and product category."
			strength = 0.75
		case record.SupportAgentID == target.SupportAgentID && record.ReturnReason == target.ReturnReason:
			relationType = "SAME_AGENT_REASON"
			reason = "Same support agent and return reason."
			strength = 0.6
		default:
			continue
		}

		items = append(items, domain.RelatedReturn{
			ReturnID:       record.ReturnID,
			RelationType:   relationType,
			Reason:         reason,
			Strength:       strength,
			CustomerID:     record.CustomerID,
			SupportAgentID: record.SupportAgentID,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Strength == items[j].Strength {
			return items[i].ReturnID < items[j].ReturnID
		}
		return items[i].Strength > items[j].Strength
	})

	truncated := false
	if limit > 0 && len(items) > limit {
		items = items[:limit]
		truncated = true
	}

	return relatedResult{Items: items, Truncated: truncated}
}

func rebuildResponse(state *datasetState, jobID, status string) *domain.DatasetRebuildResponse {
	return &domain.DatasetRebuildResponse{
		DatasetID:      state.metadata.DatasetID,
		JobID:          jobID,
		Status:         status,
		RelationsCount: state.relations,
		FeaturesCount:  len(state.features),
		RecordsCount:   state.metadata.RecordsCount,
		SchemaVersion:  state.metadata.SchemaVersion,
		FeatureVersion: state.metadata.FeatureVersion,
		CalculatedAt:   state.metadata.CalculatedAt,
	}
}

func (s *Service) publishRelationsBuilt(ctx context.Context, datasetID, jobID string, state *datasetState) error {
	if s.publisher == nil {
		return nil
	}

	return s.publisher.PublishRelationsBuilt(ctx, RelationsBuiltEvent{
		DatasetID:      datasetID,
		JobID:          jobID,
		RecordsPath:    state.metadata.Source,
		RecordsCount:   state.metadata.RecordsCount,
		RelationsCount: state.relations,
		FeaturesCount:  len(state.features),
		SchemaVersion:  state.metadata.SchemaVersion,
		FeatureVersion: state.metadata.FeatureVersion,
		PublishedAt:    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Service) publishPipelineFailed(ctx context.Context, event NormalizedDatasetEvent, err error) error {
	if s.publisher == nil {
		return nil
	}

	return s.publisher.PublishPipelineFailed(ctx, PipelineFailedEvent{
		DatasetID: event.DatasetID,
		JobID:     event.JobID,
		Stage:     "RELATIONS",
		Message:   err.Error(),
		FailedAt:  time.Now().UTC().Format(time.RFC3339),
	})
}

func findRecord(records []domain.NormalizedReturnRecord, returnID string) (domain.NormalizedReturnRecord, bool) {
	for _, record := range records {
		if record.ReturnID == returnID {
			return record, true
		}
	}
	return domain.NormalizedReturnRecord{}, false
}

func loadRecords(options Options) ([]domain.NormalizedReturnRecord, error) {
	datasetID := options.DatasetID
	if datasetID == "" {
		datasetID = defaultDatasetID
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
	if strings.HasPrefix(path, "file://") {
		path = strings.TrimPrefix(path, "file://")
	}

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
		index[strings.TrimSpace(name)] = i
	}
	if err := validateHeader(index); err != nil {
		return nil, err
	}

	records := make([]domain.NormalizedReturnRecord, 0)
	line := 1
	for {
		line++
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		record, err := parseNormalizedReturnRecord(datasetID, row, index)
		if err != nil {
			return nil, fmt.Errorf("%w: line %d: %v", ErrInvalidDataset, line, err)
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("%w: normalized refund dataset is empty", ErrInvalidDataset)
	}
	if err := validateRecords(datasetID, records); err != nil {
		return nil, err
	}

	return records, nil
}

func validateHeader(index map[string]int) error {
	required := []string{
		"order_id",
		"customer_id",
		"return_id",
		"support_agent_id",
		"order_amount",
		"refund_amount",
		"product_category",
		"return_reason",
		"decision",
		"manual_override",
		"decision_time_minutes",
	}
	missing := make([]string, 0)
	for _, column := range required {
		if _, ok := index[column]; !ok {
			missing = append(missing, column)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing required columns: %s", ErrInvalidDataset, strings.Join(missing, ", "))
	}
	return nil
}

func validateRecords(datasetID string, records []domain.NormalizedReturnRecord) error {
	if len(records) == 0 {
		return fmt.Errorf("%w: dataset %s has no records", ErrInvalidDataset, datasetID)
	}
	seen := map[string]struct{}{}
	for _, record := range records {
		if record.DatasetID != datasetID {
			return fmt.Errorf("%w: record %s belongs to dataset %s, expected %s", ErrInvalidDataset, record.ReturnID, record.DatasetID, datasetID)
		}
		if record.ReturnID == "" || record.CustomerID == "" || record.OrderID == "" || record.SupportAgentID == "" {
			return fmt.Errorf("%w: required identifier is empty", ErrInvalidDataset)
		}
		if _, exists := seen[record.ReturnID]; exists {
			return fmt.Errorf("%w: duplicate returnId %s in dataset %s", ErrInvalidDataset, record.ReturnID, datasetID)
		}
		seen[record.ReturnID] = struct{}{}
	}
	return nil
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
	returnID := readString(row, index, "return_id")

	return domain.NormalizedReturnRecord{
		DatasetID:           datasetID,
		ReturnID:            returnID,
		CustomerID:          readString(row, index, "customer_id"),
		OrderID:             readString(row, index, "order_id"),
		SupportAgentID:      readString(row, index, "support_agent_id"),
		ProductCategory:     readString(row, index, "product_category"),
		ReturnReason:        readString(row, index, "return_reason"),
		DecisionID:          fmt.Sprintf("decision_%s", strings.TrimPrefix(returnID, "return_")),
		DecisionStatus:      strings.ToUpper(readString(row, index, "decision")),
		RefundAmount:        refundAmount,
		OrderAmount:         orderAmount,
		ManualOverride:      manualOverride,
		DecisionTimeMinutes: decisionTimeMinutes,
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
	return strconv.ParseFloat(readString(row, index, column), 64)
}

func parseBool(row []string, index map[string]int, column string) (bool, error) {
	return strconv.ParseBool(readString(row, index, column))
}

func parseInt(row []string, index map[string]int, column string) (int, error) {
	return strconv.Atoi(readString(row, index, column))
}

func graphNode(id, nodeType, label, summary string, data map[string]interface{}) domain.GraphNode {
	return domain.GraphNode{ID: id, Type: nodeType, Label: label, Summary: summary, Data: data}
}

func graphEdge(from, to, edgeType, label string, count int, weight float64, reason string) domain.GraphEdge {
	return domain.GraphEdge{From: from, To: to, Type: edgeType, Label: label, Count: count, Weight: weight, Reason: reason}
}

func dedupeNodes(nodes []domain.GraphNode) []domain.GraphNode {
	seen := map[string]struct{}{}
	result := make([]domain.GraphNode, 0, len(nodes))
	for _, node := range nodes {
		if _, ok := seen[node.ID]; ok {
			continue
		}
		seen[node.ID] = struct{}{}
		result = append(result, node)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func dedupeEdges(edges []domain.GraphEdge) []domain.GraphEdge {
	seen := map[string]struct{}{}
	result := make([]domain.GraphEdge, 0, len(edges))
	for _, edge := range edges {
		key := edge.From + "|" + edge.To + "|" + edge.Type
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, edge)
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := result[i].From + result[i].To + result[i].Type
		right := result[j].From + result[j].To + result[j].Type
		return left < right
	})
	return result
}

func relationLabel(relationType string) string {
	switch relationType {
	case "CUSTOMER_AGENT_PAIR":
		return "same customer-agent pair"
	case "SAME_CUSTOMER":
		return "same customer"
	case "SIMILAR_AGENT_REASON_CATEGORY":
		return "same agent/reason/category"
	case "SAME_AGENT_REASON":
		return "same agent/reason"
	default:
		return strings.ToLower(strings.ReplaceAll(relationType, "_", " "))
	}
}

func itemCount(item domain.RelatedReturn) int {
	if item.Strength >= 1 {
		return 2
	}
	return 1
}

func fallbackTestDataset() []domain.NormalizedReturnRecord {
	return []domain.NormalizedReturnRecord{
		{DatasetID: defaultDatasetID, ReturnID: "return_3041", CustomerID: "customer_880", OrderID: "order_9101", SupportAgentID: "agent_017", ProductCategory: "electronics", ReturnReason: "item_not_as_described", DecisionID: "decision_7001", DecisionStatus: approvedDecision, RefundAmount: 420, OrderAmount: 520, ManualOverride: true, DecisionTimeMinutes: 65},
		{DatasetID: defaultDatasetID, ReturnID: "return_3006", CustomerID: "customer_880", OrderID: "order_9066", SupportAgentID: "agent_017", ProductCategory: "electronics", ReturnReason: "item_not_as_described", DecisionID: "decision_6966", DecisionStatus: approvedDecision, RefundAmount: 310, OrderAmount: 340, DecisionTimeMinutes: 73},
		{DatasetID: defaultDatasetID, ReturnID: "return_3022", CustomerID: "customer_880", OrderID: "order_9082", SupportAgentID: "agent_019", ProductCategory: "apparel", ReturnReason: "wrong_size", DecisionID: "decision_6982", DecisionStatus: "REJECTED", RefundAmount: 0, OrderAmount: 95.50, DecisionTimeMinutes: 103},
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
		signals = append(signals, fmt.Sprintf("Customer has %d refund requests.", customerReturnCount))
	}
	if agentDecisionCount > 0 && ratio(agentApprovedCount, agentDecisionCount) >= 0.9 {
		signals = append(signals, fmt.Sprintf("Support agent approval rate is %.0f%%.", ratio(agentApprovedCount, agentDecisionCount)*100))
	}
	if customerAgentPairCount >= 3 {
		signals = append(signals, fmt.Sprintf("Customer and support agent interacted on %d return requests.", customerAgentPairCount))
	}
	if sameReasonRefundCount >= 5 {
		signals = append(signals, fmt.Sprintf("%d returns handled by the same agent share this return reason.", sameReasonRefundCount))
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
		signals = append(signals, "No strong relation signal crossed the current thresholds.")
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

func limitStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[:limit]
}
