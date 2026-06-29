package com.antifrod.scoring.service

import com.antifrod.scoring.model.AgentRiskSummary
import com.antifrod.scoring.model.RecalculateResponse
import com.antifrod.scoring.model.RefundApprovalDetailsResponse
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
    private val refundDatasetRepository: RefundDatasetRepository,
    private val featureProvider: FeatureProvider,
    private val riskRuleEngine: RiskRuleEngine
) {

    fun getSuspiciousApprovals(datasetId: String): List<SuspiciousRefundApproval> {
        val records = findRecords(datasetId)

        return records
            .map { record -> buildRiskScore(datasetId, record, records) }
            .filter { riskScore -> riskScore.riskScore >= 31 }
            .sortedWith(
                compareByDescending<RefundApprovalRiskScore> { it.riskScore }
                    .thenBy { it.returnId }
            )
            .map { riskScore ->
                val record = records.first { it.returnId == riskScore.returnId }
                SuspiciousRefundApproval(
                    datasetId = datasetId,
                    returnId = riskScore.returnId,
                    orderId = riskScore.orderId,
                    customerId = riskScore.customerId,
                    supportAgentId = riskScore.supportAgentId,
                    refundAmount = record.refundAmount,
                    orderAmount = record.orderAmount,
                    decision = record.decision,
                    riskScore = riskScore.riskScore,
                    riskLevel = riskScore.riskLevel,
                    topReason = riskScore.topReason,
                    reasons = riskScore.reasons,
                    calculatedAt = riskScore.calculatedAt
                )
            }
    }

    fun getReturnRisk(returnId: String): RefundApprovalRiskScore {
        return getReturnRisk("demo", returnId)
    }

    fun getReturnRisk(datasetId: String, returnId: String): RefundApprovalRiskScore {
        val records = findRecords(datasetId)

        val record = records.firstOrNull { it.returnId == returnId }
            ?: throw ScoringNotFoundException("Return approval was not found: $returnId in dataset: $datasetId")

        return buildRiskScore(datasetId, record, records)
    }

    fun getAgentRiskSummary(agentId: String): AgentRiskSummary {
        return getAgentRiskSummary("demo", agentId)
    }

    fun getAgentRiskSummary(datasetId: String, agentId: String): AgentRiskSummary {
        val records = findRecords(datasetId)
        val agentRecords = records.filter { it.supportAgentId == agentId }

        if (agentRecords.isEmpty()) {
            return AgentRiskSummary(
                datasetId = datasetId,
                agentId = agentId,
                totalApprovals = 0,
                totalReturns = 0,
                suspiciousApprovalsCount = 0,
                highRiskCount = 0,
                criticalRiskCount = 0,
                averageRiskScore = 0.0,
                approvalRate = 0.0,
                topRiskReasons = emptyList(),
                highRiskApprovalsCount = 0,
                criticalRiskApprovalsCount = 0,
                topReason = "No refund approvals found for this support agent",
                calculatedAt = Instant.now()
            )
        }

        val riskScores = agentRecords.map { record -> buildRiskScore(datasetId, record, records) }
        val suspiciousScores = riskScores.filter { it.riskScore >= 31 }

        val topRiskReasons = suspiciousScores
            .flatMap { it.reasons }
            .groupBy { it.type }
            .map { (_, reasons) -> reasons.first() to reasons.size }
            .sortedWith(
                compareByDescending<Pair<RiskReason, Int>> { it.second }
                    .thenBy { riskRuleEngine.reasonPriority(it.first.type) }
                    .thenBy { it.first.type }
            )
            .map { it.first }

        val approvalRate = agentRecords.count { it.decision == "APPROVED" }.toDouble() / agentRecords.size.toDouble()

        return AgentRiskSummary(
            datasetId = datasetId,
            agentId = agentId,
            totalApprovals = agentRecords.count { it.decision == "APPROVED" },
            totalReturns = agentRecords.size,
            suspiciousApprovalsCount = suspiciousScores.size,
            highRiskCount = riskScores.count { it.riskLevel == RiskLevel.HIGH },
            criticalRiskCount = riskScores.count { it.riskLevel == RiskLevel.CRITICAL },
            averageRiskScore = riskScores.map { it.riskScore }.average(),
            approvalRate = approvalRate,
            topRiskReasons = topRiskReasons.take(3),
            highRiskApprovalsCount = riskScores.count { it.riskLevel == RiskLevel.HIGH },
            criticalRiskApprovalsCount = riskScores.count { it.riskLevel == RiskLevel.CRITICAL },
            topReason = topRiskReasons.firstOrNull()?.message ?: "No significant risk factors detected.",
            calculatedAt = Instant.now()
        )
    }

    fun recalculateDataset(datasetId: String): RecalculateResponse {
        val records = findRecords(datasetId)
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
        val records = findRecords(datasetId)

        val record = records.firstOrNull { it.returnId == returnId }
            ?: throw ScoringNotFoundException("Return approval was not found: $returnId in dataset: $datasetId")

        val risk = buildRiskScore(datasetId, record, records)
        val features = featureProvider.buildFeatures(record, records)

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
            relationFeatures = features,
            calculatedAt = risk.calculatedAt
        )
    }

    private fun buildRiskScore(
        datasetId: String,
        record: RefundApprovalRecord,
        allRecords: List<RefundApprovalRecord>
    ): RefundApprovalRiskScore {
        val features = featureProvider.buildFeatures(record, allRecords)
        val reasons = riskRuleEngine.calculateReasons(features)
        val score = riskRuleEngine.calculateScore(reasons)

        return RefundApprovalRiskScore(
            returnId = record.returnId,
            orderId = record.orderId,
            customerId = record.customerId,
            supportAgentId = record.supportAgentId,
            datasetId = datasetId,
            riskScore = score,
            riskLevel = riskRuleEngine.resolveRiskLevel(score),
            topReason = riskRuleEngine.topReason(reasons),
            reasons = reasons,
            calculatedAt = Instant.now()
        )
    }

    private fun findRecords(datasetId: String): List<RefundApprovalRecord> {
        return refundDatasetRepository.findByDatasetId(datasetId)
            .takeIf { it.isNotEmpty() }
            ?: throw ScoringNotFoundException("Dataset was not found: $datasetId")
    }
}
