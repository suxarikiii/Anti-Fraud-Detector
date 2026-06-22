package com.antifrod.scoring.service

import com.antifrod.scoring.model.AgentRiskSummary
import com.antifrod.scoring.model.RecalculateResponse
import com.antifrod.scoring.model.RefundApprovalDetailsResponse
import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RefundApprovalRecord
import com.antifrod.scoring.model.RefundApprovalRiskScore
import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.model.RiskReason
import com.antifrod.scoring.model.SuspiciousRefundApproval
import com.antifrod.scoring.repository.RefundDatasetRepository
import org.springframework.stereotype.Service
import java.time.Instant

@Service
class ScoringService(
    private val refundDatasetRepository: RefundDatasetRepository
) {

    fun getSuspiciousApprovals(datasetId: String): List<SuspiciousRefundApproval> {
        val records = refundDatasetRepository.findByDatasetId(datasetId)

        return records
            .map { record -> buildRiskScore(datasetId, record, records) }
            .filter { riskScore -> riskScore.riskScore >= 31 }
            .sortedByDescending { riskScore -> riskScore.riskScore }
            .map { riskScore ->
                SuspiciousRefundApproval(
                    returnId = riskScore.returnId,
                    orderId = riskScore.orderId,
                    customerId = riskScore.customerId,
                    supportAgentId = riskScore.supportAgentId,
                    refundAmount = records
                        .first { it.returnId == riskScore.returnId }
                        .refundAmount,
                    decision = records
                        .first { it.returnId == riskScore.returnId }
                        .decision,
                    riskScore = riskScore.riskScore,
                    riskLevel = riskScore.riskLevel,
                    topReason = riskScore.topReason
                )
            }
    }

    fun getReturnRisk(returnId: String): RefundApprovalRiskScore {
        val records = refundDatasetRepository.findAll()

        val record = refundDatasetRepository.findByReturnId(returnId)
            ?: error("Return approval was not found: $returnId")

        // Надо сделать на week 3
        return buildRiskScore("demo", record, records)
    }

    fun getAgentRiskSummary(agentId: String): AgentRiskSummary {
        val records = refundDatasetRepository.findAll()
        val agentRecords = records.filter { it.supportAgentId == agentId }

        if (agentRecords.isEmpty()) {
            return AgentRiskSummary(
                agentId = agentId,
                suspiciousApprovalsCount = 0,
                averageRiskScore = 0.0,
                highRiskApprovalsCount = 0,
                criticalRiskApprovalsCount = 0,
                topReason = "No refund approvals found for this support agent"
            )
        }

        val riskScores = agentRecords.map { record -> buildRiskScore("demo", record, records) }
        val suspiciousScores = riskScores.filter { it.riskScore >= 31 }

        val topReason = suspiciousScores
            .flatMap { it.reasons }
            .groupingBy { it.message }
            .eachCount()
            .maxByOrNull { it.value }
            ?.key
            ?: "No significant risk factors detected"

        return AgentRiskSummary(
            agentId = agentId,
            suspiciousApprovalsCount = suspiciousScores.size,
            averageRiskScore = riskScores.map { it.riskScore }.average(),
            highRiskApprovalsCount = riskScores.count { it.riskLevel == RiskLevel.HIGH },
            criticalRiskApprovalsCount = riskScores.count { it.riskLevel == RiskLevel.CRITICAL },
            topReason = topReason
        )
    }

    fun recalculateDataset(datasetId: String): RecalculateResponse {
        val records = refundDatasetRepository.findByDatasetId(datasetId)
        val suspiciousApprovals = records
            .map { record -> buildRiskScore(datasetId, record, records) }
            .count { riskScore -> riskScore.riskScore >= 31 }

        return RecalculateResponse(
            datasetId = datasetId,
            status = "RECALCULATION_STARTED_FOR_${records.size}_RECORDS_${suspiciousApprovals}_SUSPICIOUS_APPROVALS"
        )
    }

    fun processRelationsBuilt(datasetId: String): ScoringProcessingResult {
        val suspiciousApprovals = getSuspiciousApprovals(datasetId)

        return ScoringProcessingResult(
            suspiciousApprovalsCount = suspiciousApprovals.size
        )
    }

    fun getReturnDetails(datasetId: String, returnId: String): RefundApprovalDetailsResponse {
        val records = refundDatasetRepository.findByDatasetId(datasetId)

        val record = records.firstOrNull { it.returnId == returnId }
            ?: error("Return approval was not found: $returnId in dataset: $datasetId")

        val risk = buildRiskScore(datasetId, record, records)

        return RefundApprovalDetailsResponse(
            returnId = record.returnId,
            orderId = record.orderId,
            customerId = record.customerId,
            supportAgentId = record.supportAgentId,
            datasetId = datasetId,

            orderAmount = record.orderAmount,
            refundAmount = record.refundAmount,
            productCategory = record.productCategory,
            returnReason = record.returnReason,
            evidenceProvided = record.evidenceProvided,
            decision = record.decision,
            manualOverride = record.manualOverride,
            decisionTimeMinutes = record.decisionTimeMinutes,
            timestamp = record.timestamp,

            riskScore = risk.riskScore,
            riskLevel = risk.riskLevel,
            topReason = risk.topReason,
            reasons = risk.reasons,
            calculatedAt = risk.calculatedAt
        )
    }

    private fun buildRiskScore(
        datasetId: String,
        record: RefundApprovalRecord,
        allRecords: List<RefundApprovalRecord>
    ): RefundApprovalRiskScore {
        val features = buildFeatures(record, allRecords)
        val reasons = calculateReasons(features)
        val score = reasons.sumOf { it.scoreImpact }.coerceIn(0, 100)

        return RefundApprovalRiskScore(
            returnId = record.returnId,
            orderId = record.orderId,
            customerId = record.customerId,
            supportAgentId = record.supportAgentId,
            datasetId = datasetId,
            riskScore = score,
            riskLevel = resolveRiskLevel(score),
            topReason = reasons.firstOrNull()?.message ?: "No significant risk factors detected",
            reasons = reasons,
            calculatedAt = Instant.now()
        )
    }

    private fun buildFeatures(
        record: RefundApprovalRecord,
        allRecords: List<RefundApprovalRecord>
    ): RefundApprovalFeatures {
        val customerRecords = allRecords.filter { it.customerId == record.customerId }
        val agentRecords = allRecords.filter { it.supportAgentId == record.supportAgentId }
        val customerAgentRecords = allRecords.filter {
            it.customerId == record.customerId &&
                    it.supportAgentId == record.supportAgentId
        }

        val agentApprovedCount = agentRecords.count { it.decision == "APPROVED" }
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
            clusterSize = maxOf(customerReturnCount, customerAgentPairCount)
        )
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

        if (features.refundAmountRatio >= 0.70) {
            reasons += RiskReason(
                type = "HIGH_VALUE_REFUND",
                message = "Refund amount is unusually high compared to order amount",
                scoreImpact = 20
            )
        }

        if (features.refundAmountRatio >= 0.95) {
            reasons += RiskReason(
                type = "FULL_AMOUNT_REFUND",
                message = "Refund amount is close to full order amount",
                scoreImpact = 15
            )
        }

        if (features.decisionTimeMinutes <= 3) {
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

        if (features.agentDecisionCount >= 5 && features.agentApprovalRate > 0.85) {
            reasons += RiskReason(
                type = "AGENT_HIGH_APPROVAL_RATE",
                message = "Support agent has unusually high approval rate",
                scoreImpact = 30
            )
        }

        if (features.customerReturnCount >= 5) {
            reasons += RiskReason(
                type = "CUSTOMER_FREQUENT_RETURNS",
                message = "Customer has frequent refund requests",
                scoreImpact = 20
            )
        }

        if (features.customerAgentPairCount >= 3) {
            reasons += RiskReason(
                type = "REPEATED_AGENT_CUSTOMER_PAIR",
                message = "Same support agent repeatedly approved refunds for this customer",
                scoreImpact = 25
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
}

