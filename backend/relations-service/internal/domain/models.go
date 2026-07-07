package domain

type DatasetRebuildResponse struct {
	DatasetID      string `json:"datasetId"`
	JobID          string `json:"jobId"`
	Status         string `json:"status"`
	RelationsCount int    `json:"relationsCount"`
	FeaturesCount  int    `json:"featuresCount"`
}

type NormalizedReturnRecord struct {
	DatasetID       string  `json:"datasetId"`
	ReturnID        string  `json:"returnId"`
	CustomerID      string  `json:"customerId"`
	OrderID         string  `json:"orderId"`
	SupportAgentID  string  `json:"supportAgentId"`
	ProductCategory string  `json:"productCategory"`
	ReturnReason    string  `json:"returnReason"`
	DecisionID      string  `json:"decisionId"`
	DecisionStatus  string  `json:"decisionStatus"`
	RefundAmount    float64 `json:"refundAmount"`
	OrderAmount     float64 `json:"orderAmount"`
	ManualOverride  bool    `json:"manualOverride"`
	DecisionTimeMs  int     `json:"decisionTimeMs"`
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
	DecisionID     string  `json:"decisionId"`
	Status         string  `json:"status"`
	RefundAmount   float64 `json:"refundAmount"`
	ManualOverride bool    `json:"manualOverride"`
	DecisionTimeMs int     `json:"decisionTimeMs"`
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
	CustomerID     string           `json:"customerId"`
	SupportAgentID string           `json:"supportAgentId"`
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
