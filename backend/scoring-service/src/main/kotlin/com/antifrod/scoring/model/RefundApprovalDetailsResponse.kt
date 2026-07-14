package com.antifrod.scoring.model

import java.time.Instant

data class RefundApprovalDetailsResponse(
    val returnId: String,
    val orderId: String,
    val customerId: String,
    val supportAgentId: String,
    val datasetId: String,

    val orderAmount: Double,
    val refundAmount: Double,
    val productCategory: String,
    val returnReason: String,
    val evidenceProvided: Boolean,
    val decision: String,
    val manualOverride: Boolean,
    val decisionTimeMinutes: Int,
    val timestamp: String,

    val riskScore: Int,
    val riskLevel: RiskLevel,
    val topReason: String,
    val reasons: List<RiskReason>,
    val relationFeatures: RefundApprovalFeatures,
    val featureSource: String,
    val calculationVersion: Long,
    val calculatedAt: Instant
)
