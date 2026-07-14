package com.antifrod.scoring.service

import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RefundApprovalRecord

class CsvDerivedFeatureProvider : FeatureProvider {

    override fun buildFeatures(
        record: RefundApprovalRecord,
        allRecords: List<RefundApprovalRecord>
    ): RefundApprovalFeatures {
        val customerRecords = allRecords.filter { it.customerId == record.customerId }
        val agentRecords = allRecords.filter { it.supportAgentId == record.supportAgentId }
        val customerAgentRecords = allRecords.filter {
            it.customerId == record.customerId && it.supportAgentId == record.supportAgentId
        }

        val agentApprovedCount = agentRecords.count { it.decision == APPROVED }
        val agentDecisionCount = agentRecords.size
        val agentApprovalRate = if (agentDecisionCount == 0) {
            0.0
        } else {
            agentApprovedCount.toDouble() / agentDecisionCount.toDouble()
        }

        val refundAmountRatio = if (record.orderAmount == 0.0) {
            0.0
        } else {
            record.refundAmount / record.orderAmount
        }

        val customerReturnCount = customerRecords.size
        val customerAgentPairCount = customerAgentRecords.size
        val clusterSize = maxOf(customerReturnCount, customerAgentPairCount)

        return RefundApprovalFeatures(
            decision = record.decision,
            evidenceProvided = record.evidenceProvided,
            orderAmount = record.orderAmount,
            refundAmount = record.refundAmount,
            refundAmountRatio = refundAmountRatio,
            decisionTimeMinutes = record.decisionTimeMinutes,
            manualOverride = record.manualOverride,
            customerReturnCount = customerReturnCount,
            agentDecisionCount = agentDecisionCount,
            agentApprovalRate = agentApprovalRate,
            customerAgentPairCount = customerAgentPairCount,
            clusterSize = clusterSize,
            strongestRelationType = resolveStrongestRelationType(customerReturnCount, customerAgentPairCount, clusterSize),
            featureSource = "DEMO_CSV"
        )
    }

    private fun resolveStrongestRelationType(
        customerReturnCount: Int,
        customerAgentPairCount: Int,
        clusterSize: Int
    ): String {
        return when {
            customerAgentPairCount >= REPEATED_PAIR_THRESHOLD -> "REPEATED_AGENT_CUSTOMER_PAIR"
            clusterSize >= SUSPICIOUS_CLUSTER_THRESHOLD -> "SUSPICIOUS_CLUSTER"
            customerReturnCount >= CUSTOMER_FREQUENT_RETURNS_THRESHOLD -> "CUSTOMER_FREQUENT_RETURNS"
            else -> "NONE"
        }
    }

    private companion object {
        const val APPROVED = "APPROVED"
        const val CUSTOMER_FREQUENT_RETURNS_THRESHOLD = 5
        const val REPEATED_PAIR_THRESHOLD = 3
        const val SUSPICIOUS_CLUSTER_THRESHOLD = 5
    }
}
