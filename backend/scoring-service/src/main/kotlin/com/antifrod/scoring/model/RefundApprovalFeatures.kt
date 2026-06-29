package com.antifrod.scoring.model

data class RefundApprovalFeatures(
    val decision: String,
    val evidenceProvided: Boolean,
    val orderAmount: Double,
    val refundAmount: Double,
    val refundAmountRatio: Double,
    val decisionTimeMinutes: Int,
    val manualOverride: Boolean,

    val customerReturnCount: Int,
    val agentDecisionCount: Int,
    val agentApprovalRate: Double,
    val customerAgentPairCount: Int,

    val clusterSize: Int,
    val strongestRelationType: String,
    val featureSource: String
)
