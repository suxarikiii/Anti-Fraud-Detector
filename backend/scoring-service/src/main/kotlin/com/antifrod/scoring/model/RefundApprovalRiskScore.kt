package com.antifrod.scoring.model

import java.time.Instant

data class RefundApprovalRiskScore(
    val returnId: String,
    val orderId: String,
    val customerId: String,
    val supportAgentId: String,
    val datasetId: String,
    val riskScore: Int,
    val riskLevel: RiskLevel,
    val topReason: String,
    val reasons: List<RiskReason>,
    val featureSource: String,
    val calculationVersion: Long,
    val calculatedAt: Instant
)
