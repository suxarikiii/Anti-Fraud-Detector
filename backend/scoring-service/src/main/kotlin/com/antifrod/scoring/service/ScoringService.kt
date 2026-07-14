package com.antifrod.scoring.service

import com.antifrod.scoring.messaging.event.RelationsBuiltEvent
import com.antifrod.scoring.model.AgentRiskSummary
import com.antifrod.scoring.model.InvestigationDecision
import com.antifrod.scoring.model.InvestigationDecisionRequest
import com.antifrod.scoring.model.InvestigationOutcome
import com.antifrod.scoring.model.RecalculateResponse
import com.antifrod.scoring.model.RefundApprovalDetailsResponse
import com.antifrod.scoring.model.RefundApprovalRiskScore
import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.model.RiskReason
import com.antifrod.scoring.model.SuspiciousRefundApproval
import com.antifrod.scoring.repository.ScoringResultRepository
import com.antifrod.scoring.repository.StoredScoringResult
import org.springframework.dao.DuplicateKeyException
import org.springframework.stereotype.Service
import java.io.StringWriter
import java.time.Instant
import java.util.concurrent.ConcurrentHashMap

@Service
class ScoringService(
    private val datasetProvider: DatasetProvider,
    private val resultRepository: ScoringResultRepository,
    private val riskRuleEngine: RiskRuleEngine
) {
    private val datasetLocks = ConcurrentHashMap<String, Any>()
    fun getSuspiciousApprovals(
        datasetId: String,
        risk: RiskLevel? = null,
        agent: String? = null,
        outcome: InvestigationOutcome? = null
    ): List<SuspiciousRefundApproval> {
        val decisions = resultRepository.decisionsForDataset(datasetId)
        return ensureResults(datasetId)
            .asSequence()
            .filter { it.risk.riskScore >= 31 }
            .filter { risk == null || it.risk.riskLevel == risk }
            .filter { agent.isNullOrBlank() || it.record.supportAgentId == agent }
            .filter { outcome == null || decisions[it.record.returnId]?.outcome == outcome }
            .sortedWith(compareByDescending<StoredScoringResult> { it.risk.riskScore }.thenBy { it.record.returnId })
            .map { stored -> stored.toSuspicious(datasetId) }
            .toList()
    }

    fun getReturnRisk(returnId: String): RefundApprovalRiskScore = getReturnRisk("demo", returnId)

    fun getReturnRisk(datasetId: String, returnId: String): RefundApprovalRiskScore {
        ensureResults(datasetId)
        return resultRepository.findLatest(datasetId, returnId)?.risk
            ?: throw ScoringNotFoundException("Return approval was not found: $returnId in dataset: $datasetId")
    }

    fun getAgentRiskSummary(agentId: String): AgentRiskSummary = getAgentRiskSummary("demo", agentId)

    fun getAgentRiskSummary(datasetId: String, agentId: String): AgentRiskSummary {
        val agentResults = ensureResults(datasetId).filter { it.record.supportAgentId == agentId }
        if (agentResults.isEmpty()) {
            return AgentRiskSummary(datasetId, agentId, 0, 0, 0, 0, 0, 0.0, 0.0, emptyList(), 0, 0,
                "No refund approvals found for this support agent", Instant.now())
        }
        val suspicious = agentResults.filter { it.risk.riskScore >= 31 }
        val topReasons = suspicious.flatMap { it.risk.reasons }
            .groupingBy { it.type }.eachCount()
            .entries.sortedWith(compareByDescending<Map.Entry<String, Int>> { it.value }
                .thenBy { riskRuleEngine.reasonPriority(it.key) })
            .mapNotNull { entry -> suspicious.flatMap { it.risk.reasons }.firstOrNull { it.type == entry.key } }
        val high = agentResults.count { it.risk.riskLevel == RiskLevel.HIGH }
        val critical = agentResults.count { it.risk.riskLevel == RiskLevel.CRITICAL }
        return AgentRiskSummary(
            datasetId, agentId, agentResults.count { it.record.decision == "APPROVED" }, agentResults.size,
            suspicious.size, high, critical, agentResults.map { it.risk.riskScore }.average(),
            agentResults.count { it.record.decision == "APPROVED" }.toDouble() / agentResults.size,
            topReasons.take(3), high, critical,
            topReasons.firstOrNull()?.message ?: "No significant risk factors detected.", Instant.now()
        )
    }

    fun recalculateDataset(datasetId: String): RecalculateResponse {
        return synchronized(datasetLocks.computeIfAbsent(datasetId) { Any() }) {
            val snapshot = datasetProvider.load(datasetId)
            val calculated = calculate(snapshot)
            val version = resultRepository.saveRun(null, null, datasetId, snapshot.featureVersion, snapshot.featureSource, calculated)
            RecalculateResponse(datasetId, "RECALCULATED_${calculated.size}_RECORDS_VERSION_$version")
        }
    }

    fun processRelationsBuilt(event: RelationsBuiltEvent): ScoringProcessingResult =
        synchronized(datasetLocks.computeIfAbsent(event.datasetId) { Any() }) {
            processRelationsBuiltLocked(event)
        }

    private fun processRelationsBuiltLocked(event: RelationsBuiltEvent): ScoringProcessingResult {
        val eventKey = event.idempotencyKey()
        resultRepository.findProcessedEvent(eventKey)?.let {
            return ScoringProcessingResult(it.scoredCount, it.suspiciousCount, duplicate = true)
        }
        val snapshot = datasetProvider.load(event.datasetId)
        if (event.featureVersion > 0 && snapshot.featureVersion != event.featureVersion) {
            throw ScoringDependencyException(
                "Relations feature version changed before scoring could start",
                "FEATURE_VERSION_MISMATCH"
            )
        }
        val calculated = calculate(snapshot)
        try {
            resultRepository.saveRun(eventKey, event.jobId, event.datasetId, snapshot.featureVersion, snapshot.featureSource, calculated)
        } catch (_: DuplicateKeyException) {
            val existing = resultRepository.findProcessedEvent(eventKey)
                ?: throw ScoringDependencyException("Could not resolve duplicate scoring event", "IDEMPOTENCY_CONFLICT")
            return ScoringProcessingResult(existing.scoredCount, existing.suspiciousCount, duplicate = true)
        }
        return ScoringProcessingResult(calculated.size, calculated.count { it.risk.riskScore >= 31 })
    }

    fun processRelationsBuilt(datasetId: String): ScoringProcessingResult =
        processRelationsBuilt(RelationsBuiltEvent(datasetId = datasetId))

    fun getReturnDetails(datasetId: String, returnId: String): RefundApprovalDetailsResponse {
        ensureResults(datasetId)
        val stored = resultRepository.findLatest(datasetId, returnId)
            ?: throw ScoringNotFoundException("Return approval was not found: $returnId in dataset: $datasetId")
        val record = stored.record
        val risk = stored.risk
        return RefundApprovalDetailsResponse(
            record.returnId, record.orderId, record.customerId, record.supportAgentId, datasetId,
            record.orderAmount, record.refundAmount, record.productCategory, record.returnReason,
            record.evidenceProvided, record.decision, record.manualOverride, record.decisionTimeMinutes,
            record.timestamp, risk.riskScore, risk.riskLevel, risk.topReason, risk.reasons, stored.features,
            risk.featureSource, risk.calculationVersion, risk.calculatedAt
        )
    }

    fun getDecision(datasetId: String, returnId: String): InvestigationDecision {
        requireReturn(datasetId, returnId)
        return resultRepository.findDecision(datasetId, returnId)
            ?: throw ScoringNotFoundException("Investigation decision was not found: $returnId in dataset: $datasetId")
    }

    fun saveDecision(datasetId: String, returnId: String, request: InvestigationDecisionRequest): InvestigationDecision {
        requireReturn(datasetId, returnId)
        val existing = resultRepository.findDecision(datasetId, returnId)
        if (existing != null && !allowedTransition(existing.outcome, request.outcome)) {
            throw ScoringValidationException("Invalid investigation outcome transition: ${existing.outcome} -> ${request.outcome}")
        }
        if (request.analystId.isBlank()) throw ScoringValidationException("analystId must not be blank")
        return resultRepository.saveDecision(datasetId, returnId, request.copy(note = request.note.trim()))
    }

    fun exportCsv(
        datasetId: String,
        risk: RiskLevel? = null,
        agent: String? = null,
        outcome: InvestigationOutcome? = null
    ): String {
        val decisions = resultRepository.decisionsForDataset(datasetId)
        val rows = ensureResults(datasetId).filter { stored ->
            (risk == null || stored.risk.riskLevel == risk) &&
                (agent.isNullOrBlank() || stored.record.supportAgentId == agent) &&
                (outcome == null || decisions[stored.record.returnId]?.outcome == outcome)
        }
        val writer = StringWriter()
        writer.append('\uFEFF')
        appendCsvRow(writer, listOf("datasetId", "returnId", "orderId", "customerId", "supportAgentId",
            "orderAmount", "refundAmount", "riskScore", "riskLevel", "reasons", "featureSource",
            "calculationVersion", "calculatedAt", "analystAction", "analystOutcome", "analystNote",
            "analystId", "decisionUpdatedAt"))
        rows.forEach { stored ->
            val decision = decisions[stored.record.returnId]
            appendCsvRow(writer, listOf(
                datasetId, stored.record.returnId, stored.record.orderId, stored.record.customerId,
                stored.record.supportAgentId, stored.record.orderAmount.toString(), stored.record.refundAmount.toString(),
                stored.risk.riskScore.toString(), stored.risk.riskLevel.name,
                stored.risk.reasons.joinToString(" | ") { "${it.type}: ${it.message}" }, stored.risk.featureSource,
                stored.risk.calculationVersion.toString(), stored.risk.calculatedAt.toString(),
                decision?.action?.name.orEmpty(), decision?.outcome?.name.orEmpty(), decision?.note.orEmpty(),
                decision?.analystId.orEmpty(), decision?.updatedAt?.toString().orEmpty()
            ))
        }
        return writer.toString()
    }

    private fun ensureResults(datasetId: String): List<StoredScoringResult> {
        resultRepository.findLatest(datasetId).takeIf { it.isNotEmpty() }?.let { return it }
        return synchronized(datasetLocks.computeIfAbsent(datasetId) { Any() }) {
            resultRepository.findLatest(datasetId).takeIf { it.isNotEmpty() } ?: run {
                val snapshot = datasetProvider.load(datasetId)
                val calculated = calculate(snapshot)
                resultRepository.saveRun(null, null, datasetId, snapshot.featureVersion, snapshot.featureSource, calculated)
                resultRepository.findLatest(datasetId)
            }
        }
    }

    private fun calculate(snapshot: DatasetSnapshot): List<StoredScoringResult> {
        require(snapshot.records.isNotEmpty()) { "Dataset ${snapshot.datasetId} has no records" }
        return snapshot.records.map { record ->
            val features = snapshot.features[record.returnId]
                ?: throw ScoringDependencyException("Features are missing for return ${record.returnId}", "FEATURES_MISSING")
            val reasons = riskRuleEngine.calculateReasons(features)
            val score = riskRuleEngine.calculateScore(reasons)
            val calculatedAt = Instant.now()
            val risk = RefundApprovalRiskScore(
                record.returnId, record.orderId, record.customerId, record.supportAgentId, snapshot.datasetId,
                score, riskRuleEngine.resolveRiskLevel(score), riskRuleEngine.topReason(reasons), reasons,
                snapshot.featureSource, 0, calculatedAt
            )
            StoredScoringResult(record, risk, features)
        }
    }

    private fun StoredScoringResult.toSuspicious(datasetId: String) = SuspiciousRefundApproval(
        datasetId, record.returnId, record.orderId, record.customerId, record.supportAgentId,
        record.refundAmount, record.orderAmount, record.decision, risk.riskScore, risk.riskLevel,
        risk.topReason, risk.reasons, risk.featureSource, risk.calculationVersion, risk.calculatedAt
    )

    private fun requireReturn(datasetId: String, returnId: String) {
        ensureResults(datasetId)
        if (resultRepository.findLatest(datasetId, returnId) == null) {
            throw ScoringNotFoundException("Return approval was not found: $returnId in dataset: $datasetId")
        }
    }

    private fun allowedTransition(from: InvestigationOutcome, to: InvestigationOutcome): Boolean = when (from) {
        InvestigationOutcome.OPEN -> true
        InvestigationOutcome.NEEDS_MORE_INFO -> to != InvestigationOutcome.OPEN
        InvestigationOutcome.CONFIRMED_FRAUD, InvestigationOutcome.FALSE_POSITIVE, InvestigationOutcome.RESOLVED -> from == to
    }

    private fun appendCsvRow(writer: StringWriter, values: List<String>) {
        writer.append(values.joinToString(",") { value -> "\"${value.replace("\"", "\"\"")}\"" })
        writer.append("\r\n")
    }
}
