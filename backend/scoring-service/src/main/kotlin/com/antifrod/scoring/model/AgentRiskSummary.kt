package com.antifrod.scoring.model

data class AgentRiskSummary(
    val agentId: String,
    val suspiciousApprovalsCount: Int,
    val averageRiskScore: Double,
    val highRiskApprovalsCount: Int,
    val criticalRiskApprovalsCount: Int,
    val topReason: String
)