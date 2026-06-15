package com.antifrod.scoring.model

data class RefundApprovalRecord(
    val orderId: String,
    val customerId: String,
    val returnId: String,
    val supportAgentId: String,
    val orderAmount: Double,
    val refundAmount: Double,
    val productCategory: String,
    val returnReason: String,
    val evidenceProvided: Boolean,
    val decision: String,
    val manualOverride: Boolean,
    val decisionTimeMinutes: Int,
    val timestamp: String
)