package com.antifrod.scoring.service

import com.antifrod.scoring.model.AgentRiskSummary
import com.antifrod.scoring.model.RecalculateResponse
import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RefundApprovalRiskScore
import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.model.RiskReason
import com.antifrod.scoring.model.SuspiciousRefundApproval
import org.springframework.stereotype.Service
import java.time.Instant

@Service
class ScoringService {

    fun getSuspiciousApprovals(datasetId: String): List<SuspiciousRefundApproval> {
        return listOf(
            SuspiciousRefundApproval(
                returnId = "return_123",
                orderId = "order_456",
                customerId = "customer_789",
                supportAgentId = "agent_001",
                refundAmount = 11500.0,
                decision = "APPROVED",
                riskScore = 84,
                riskLevel = RiskLevel.CRITICAL,
                topReason = "Refund approved without evidence for a high-value order"
            ),
            SuspiciousRefundApproval(
                returnId = "return_222",
                orderId = "order_777",
                customerId = "customer_555",
                supportAgentId = "agent_001",
                refundAmount = 6200.0,
                decision = "APPROVED",
                riskScore = 67,
                riskLevel = RiskLevel.HIGH,
                topReason = "Support agent has unusually high approval rate"
            ),
            SuspiciousRefundApproval(
                returnId = "return_333",
                orderId = "order_888",
                customerId = "customer_999",
                supportAgentId = "agent_002",
                refundAmount = 1200.0,
                decision = "APPROVED",
                riskScore = 42,
                riskLevel = RiskLevel.MEDIUM,
                topReason = "Customer has frequent refund requests"
            )
        )
    }

    fun getReturnRisk(returnId: String): RefundApprovalRiskScore {
        val datasetId = "demo"

        val features = mockFeaturesForReturn(returnId)
        val reasons = calculateReasons(features)
        val score = reasons.sumOf { it.scoreImpact }.coerceIn(0, 100)
        val riskLevel = resolveRiskLevel(score)

        return RefundApprovalRiskScore(
            returnId = returnId,
            orderId = mockOrderId(returnId),
            customerId = mockCustomerId(returnId),
            supportAgentId = mockSupportAgentId(returnId),
            datasetId = datasetId,
            riskScore = score,
            riskLevel = riskLevel,
            topReason = reasons.firstOrNull()?.message ?: "No significant risk factors detected",
            reasons = reasons,
            calculatedAt = Instant.now()
        )
    }

    fun getAgentRiskSummary(agentId: String): AgentRiskSummary {
        return when (agentId) {
            "agent_001" -> AgentRiskSummary(
                agentId = agentId,
                suspiciousApprovalsCount = 12,
                averageRiskScore = 76.5,
                highRiskApprovalsCount = 8,
                criticalRiskApprovalsCount = 3,
                topReason = "Support agent has unusually high approval rate"
            )

            else -> AgentRiskSummary(
                agentId = agentId,
                suspiciousApprovalsCount = 3,
                averageRiskScore = 41.0,
                highRiskApprovalsCount = 1,
                criticalRiskApprovalsCount = 0,
                topReason = "Several medium-risk refund approvals detected"
            )
        }
    }

    fun recalculateDataset(datasetId: String): RecalculateResponse {
        return RecalculateResponse(
            datasetId = datasetId,
            status = "RECALCULATION_STARTED"
        )
    }

    fun processRelationsBuilt(datasetId: String): ScoringProcessingResult {
        val suspiciousApprovals = getSuspiciousApprovals(datasetId)

        return ScoringProcessingResult(
            suspiciousApprovalsCount = suspiciousApprovals.size
        )
    }

    private fun mockFeaturesForReturn(returnId: String): RefundApprovalFeatures {
        return when (returnId) {
            "return_123" -> RefundApprovalFeatures(
                decision = "APPROVED",
                evidenceProvided = false,
                orderAmount = 12000.0,
                refundAmount = 11500.0,
                decisionTimeMinutes = 1,
                manualOverride = true,
                agentApprovalRate = 0.91,
                customerReturnCount = 6,
                customerAgentPairCount = 4,
                clusterSize = 7
            )

            "return_222" -> RefundApprovalFeatures(
                decision = "APPROVED",
                evidenceProvided = true,
                orderAmount = 8000.0,
                refundAmount = 6200.0,
                decisionTimeMinutes = 4,
                manualOverride = false,
                agentApprovalRate = 0.89,
                customerReturnCount = 2,
                customerAgentPairCount = 1,
                clusterSize = 3
            )

            else -> RefundApprovalFeatures(
                decision = "APPROVED",
                evidenceProvided = true,
                orderAmount = 2500.0,
                refundAmount = 1200.0,
                decisionTimeMinutes = 10,
                manualOverride = false,
                agentApprovalRate = 0.62,
                customerReturnCount = 5,
                customerAgentPairCount = 1,
                clusterSize = 2
            )
        }
    }

    private fun calculateReasons(features: RefundApprovalFeatures): List<RiskReason> {
        val reasons = mutableListOf<RiskReason>()

        if (features.decision == "APPROVED" && !features.evidenceProvided) {
            reasons += RiskReason(
                type = "NO_EVIDENCE",
                message = "Refund was approved without required evidence",
                scoreImpact = 25
            )
        }

        if (features.refundAmount >= features.orderAmount * 0.7) {
            reasons += RiskReason(
                type = "HIGH_VALUE_REFUND",
                message = "Refund amount is unusually high compared to order amount",
                scoreImpact = 20
            )
        }

        if (features.refundAmount >= features.orderAmount * 0.95) {
            reasons += RiskReason(
                type = "FULL_AMOUNT_REFUND",
                message = "Refund amount is close to full order amount",
                scoreImpact = 15
            )
        }

        if (features.decisionTimeMinutes <= 2) {
            reasons += RiskReason(
                type = "FAST_APPROVAL",
                message = "Refund was approved very quickly",
                scoreImpact = 15
            )
        }

        if (features.manualOverride) {
            reasons += RiskReason(
                type = "MANUAL_OVERRIDE",
                message = "Manual override was used for this refund approval",
                scoreImpact = 20
            )
        }

        if (features.agentApprovalRate > 0.85) {
            reasons += RiskReason(
                type = "AGENT_HIGH_APPROVAL_RATE",
                message = "Support agent has unusually high approval rate",
                scoreImpact = 25
            )
        }

        if (features.customerReturnCount >= 5) {
            reasons += RiskReason(
                type = "CUSTOMER_FREQUENT_RETURNS",
                message = "Customer has frequent refund requests",
                scoreImpact = 15
            )
        }

        if (features.customerAgentPairCount >= 3) {
            reasons += RiskReason(
                type = "REPEATED_AGENT_CUSTOMER_PAIR",
                message = "Same support agent repeatedly approved refunds for this customer",
                scoreImpact = 20
            )
        }

        if (features.clusterSize >= 5) {
            reasons += RiskReason(
                type = "SUSPICIOUS_CLUSTER",
                message = "Refund approval belongs to a suspicious relation cluster",
                scoreImpact = 25
            )
        }

        return reasons
    }

    private fun resolveRiskLevel(score: Int): RiskLevel {
        return when {
            score >= 81 -> RiskLevel.CRITICAL
            score >= 61 -> RiskLevel.HIGH
            score >= 31 -> RiskLevel.MEDIUM
            else -> RiskLevel.LOW
        }
    }

    private fun mockOrderId(returnId: String): String {
        return when (returnId) {
            "return_123" -> "order_456"
            "return_222" -> "order_777"
            "return_333" -> "order_888"
            else -> "order_demo"
        }
    }

    private fun mockCustomerId(returnId: String): String {
        return when (returnId) {
            "return_123" -> "customer_789"
            "return_222" -> "customer_555"
            "return_333" -> "customer_999"
            else -> "customer_demo"
        }
    }

    private fun mockSupportAgentId(returnId: String): String {
        return when (returnId) {
            "return_123" -> "agent_001"
            "return_222" -> "agent_001"
            "return_333" -> "agent_002"
            else -> "agent_demo"
        }
    }
}
