package com.antifrod.scoring.model

data class SuspiciousRefundApproval(
    val returnId: String,
    val orderId: String,
    val customerId: String,
    val supportAgentId: String,
    val refundAmount: Double,
    val decision: String,
    val riskScore: Int,
    val riskLevel: RiskLevel,
    val topReason: String
)