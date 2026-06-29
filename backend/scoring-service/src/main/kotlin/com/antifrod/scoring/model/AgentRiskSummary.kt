package com.antifrod.scoring.model

data class AgentRiskSummary(
    val datasetId: String,
    val agentId: String,
    val totalApprovals: Int,
    val totalReturns: Int,
    val suspiciousApprovalsCount: Int,
    val highRiskCount: Int,
    val criticalRiskCount: Int,
    val averageRiskScore: Double,
    val approvalRate: Double,
    val topRiskReasons: List<RiskReason>,
    val highRiskApprovalsCount: Int,
    val criticalRiskApprovalsCount: Int,
    val topReason: String,
    val calculatedAt: java.time.Instant
)
