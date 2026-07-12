package domain

type DatasetRebuildResponse struct {
	DatasetID      string `json:"datasetId"`
	JobID          string `json:"jobId"`
	Status         string `json:"status"`
	RelationsCount int    `json:"relationsCount"`
	FeaturesCount  int    `json:"featuresCount"`
	RecordsCount   int    `json:"recordsCount"`
	SchemaVersion  string `json:"schemaVersion"`
	FeatureVersion int64  `json:"featureVersion"`
	CalculatedAt   string `json:"calculatedAt"`
}

type NormalizedReturnRecord struct {
	DatasetID           string  `json:"datasetId"`
	ReturnID            string  `json:"returnId"`
	CustomerID          string  `json:"customerId"`
	OrderID             string  `json:"orderId"`
	SupportAgentID      string  `json:"supportAgentId"`
	ProductCategory     string  `json:"productCategory"`
	ReturnReason        string  `json:"returnReason"`
	DecisionID          string  `json:"decisionId"`
	DecisionStatus      string  `json:"decisionStatus"`
	RefundAmount        float64 `json:"refundAmount"`
	OrderAmount         float64 `json:"orderAmount"`
	ManualOverride      bool    `json:"manualOverride"`
	DecisionTimeMinutes int     `json:"decisionTimeMinutes"`
}

type DatasetMetadata struct {
	DatasetID      string `json:"datasetId"`
	RecordsCount   int    `json:"recordsCount"`
	SchemaVersion  string `json:"schemaVersion"`
	Source         string `json:"source"`
	FeatureVersion int64  `json:"featureVersion"`
	LoadedAt       string `json:"loadedAt"`
	CalculatedAt   string `json:"calculatedAt"`
}

type ReturnRelations struct {
	ReturnID        string          `json:"returnId"`
	CustomerID      string          `json:"customerId"`
	OrderID         string          `json:"orderId"`
	SupportAgentID  string          `json:"supportAgentId"`
	ProductCategory string          `json:"productCategory"`
	Decision        SupportDecision `json:"decision"`
	Relations       []GraphRelation `json:"relations"`
}

type GraphRelation struct {
	From string `json:"from"`
	Type string `json:"type"`
	To   string `json:"to"`
}

type SupportDecision struct {
	DecisionID          string  `json:"decisionId"`
	Status              string  `json:"status"`
	RefundAmount        float64 `json:"refundAmount"`
	ManualOverride      bool    `json:"manualOverride"`
	DecisionTimeMinutes int     `json:"decisionTimeMinutes"`
}

type CustomerHistory struct {
	CustomerID      string           `json:"customerId"`
	OrdersCount     int              `json:"ordersCount"`
	ReturnCount     int              `json:"returnCount"`
	ApprovedRefunds int              `json:"approvedRefunds"`
	RecentReturns   []ReturnListItem `json:"recentReturns"`
	LinkedAgents    []LinkedAgent    `json:"linkedAgents"`
}

type ReturnListItem struct {
	ReturnID       string  `json:"returnId"`
	OrderID        string  `json:"orderId"`
	Reason         string  `json:"reason"`
	Category       string  `json:"category"`
	RefundAmount   float64 `json:"refundAmount"`
	DecisionStatus string  `json:"decisionStatus"`
	SupportAgentID string  `json:"supportAgentId"`
}

type LinkedAgent struct {
	SupportAgentID string `json:"supportAgentId"`
	PairCount      int    `json:"pairCount"`
}

type AgentSummary struct {
	SupportAgentID            string  `json:"supportAgentId"`
	DecisionsCount            int     `json:"decisionsCount"`
	ApprovalRate              float64 `json:"approvalRate"`
	HighValueApprovalCount    int     `json:"highValueApprovalCount"`
	ManualOverrideCount       int     `json:"manualOverrideCount"`
	RepeatedCustomerPairCount int     `json:"repeatedCustomerPairCount"`
	TopRiskyCategory          string  `json:"topRiskyCategory"`
}

type ReturnFeaturesResponse struct {
	ReturnID       string           `json:"returnId"`
	DatasetID      string           `json:"datasetId"`
	CustomerID     string           `json:"customerId"`
	SupportAgentID string           `json:"supportAgentId"`
	FeatureVersion int64            `json:"featureVersion"`
	CalculatedAt   string           `json:"calculatedAt"`
	Features       RelationFeatures `json:"features"`
}

type RelationFeatures struct {
	CustomerReturnCount           int      `json:"customerReturnCount"`
	CustomerApprovedRefundCount   int      `json:"customerApprovedRefundCount"`
	AgentApprovalRate             float64  `json:"agentApprovalRate"`
	AgentManualOverrideRate       float64  `json:"agentManualOverrideRate"`
	AgentHighValueApprovalCount   int      `json:"agentHighValueApprovalCount"`
	CustomerAgentPairCount        int      `json:"customerAgentPairCount"`
	AgentCustomerInteractionCount int      `json:"agentCustomerInteractionCount"`
	CategoryRefundRate            float64  `json:"categoryRefundRate"`
	RefundAmountRatio             float64  `json:"refundAmountRatio"`
	SimilarReturnsCount           int      `json:"similarReturnsCount"`
	SameReasonRefundCount         int      `json:"sameReasonRefundCount"`
	ClusterSize                   int      `json:"clusterSize"`
	StrongestRelationType         string   `json:"strongestRelationType"`
	TopRelatedReturns             []string `json:"topRelatedReturns"`
	ExplanationSummary            string   `json:"explanationSummary"`
	ExplanationSignals            []string `json:"explanationSignals"`
}

type RelatedReturn struct {
	ReturnID       string  `json:"returnId"`
	RelationType   string  `json:"relationType"`
	Reason         string  `json:"reason"`
	Strength       float64 `json:"strength"`
	CustomerID     string  `json:"customerId"`
	SupportAgentID string  `json:"supportAgentId"`
}

type RelatedReturnsResponse struct {
	DatasetID      string          `json:"datasetId"`
	ReturnID       string          `json:"returnId"`
	RelatedReturns []RelatedReturn `json:"relatedReturns"`
	Limit          int             `json:"limit"`
	Truncated      bool            `json:"truncated"`
}

type CustomerBehaviorSummary struct {
	DatasetID           string           `json:"datasetId"`
	CustomerID          string           `json:"customerId"`
	ReturnCount         int              `json:"returnCount"`
	ApprovedRefundCount int              `json:"approvedRefundCount"`
	TotalRefundAmount   float64          `json:"totalRefundAmount"`
	AverageRefundRatio  float64          `json:"averageRefundRatio"`
	RelatedAgents       []LinkedAgent    `json:"relatedAgents"`
	RecentReturns       []ReturnListItem `json:"recentReturns"`
}

type RankedAgentSummary struct {
	SupportAgentID            string  `json:"supportAgentId"`
	DecisionsCount            int     `json:"decisionsCount"`
	ApprovalRate              float64 `json:"approvalRate"`
	ManualOverrideRate        float64 `json:"manualOverrideRate"`
	HighValueApprovalCount    int     `json:"highValueApprovalCount"`
	RepeatedCustomerPairCount int     `json:"repeatedCustomerPairCount"`
	AverageClusterSize        float64 `json:"averageClusterSize"`
	TopRiskyCategory          string  `json:"topRiskyCategory"`
}

type RankedAgentsResponse struct {
	DatasetID string               `json:"datasetId"`
	Agents    []RankedAgentSummary `json:"agents"`
	Limit     int                  `json:"limit"`
	Sort      string               `json:"sort"`
}

type GraphProjectionResponse struct {
	DatasetID string      `json:"datasetId"`
	ReturnID  string      `json:"returnId"`
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
	Limit     int         `json:"limit"`
	Truncated bool        `json:"truncated"`
}

type GraphNode struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Label   string                 `json:"label"`
	Summary string                 `json:"summary"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type GraphEdge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Type   string  `json:"type"`
	Label  string  `json:"label"`
	Count  int     `json:"count,omitempty"`
	Weight float64 `json:"weight,omitempty"`
	Reason string  `json:"reason"`
}
