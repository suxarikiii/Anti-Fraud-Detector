package com.antifrod.scoring.model

data class SuspiciousRefundApproval(
    val datasetId: String,
    val returnId: String,
    val orderId: String,
    val customerId: String,
    val supportAgentId: String,
    val refundAmount: Double,
    val orderAmount: Double,
    val decision: String,
    val riskScore: Int,
    val riskLevel: RiskLevel,
    val topReason: String,
    val reasons: List<RiskReason>,
    val featureSource: String,
    val calculationVersion: Long,
    val calculatedAt: java.time.Instant
)
