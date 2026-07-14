package com.antifrod.scoring.repository

import com.antifrod.scoring.model.InvestigationAction
import com.antifrod.scoring.model.InvestigationDecision
import com.antifrod.scoring.model.InvestigationDecisionRequest
import com.antifrod.scoring.model.InvestigationOutcome
import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RefundApprovalRecord
import com.antifrod.scoring.model.RefundApprovalRiskScore
import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.model.RiskReason
import com.fasterxml.jackson.databind.ObjectMapper
import org.springframework.dao.DuplicateKeyException
import org.springframework.jdbc.core.JdbcTemplate
import org.springframework.stereotype.Repository
import org.springframework.transaction.annotation.Transactional
import java.sql.ResultSet
import java.sql.Timestamp
import java.time.Instant

data class StoredScoringResult(
    val record: RefundApprovalRecord,
    val risk: RefundApprovalRiskScore,
    val features: RefundApprovalFeatures
)

data class ProcessedScoringEvent(
    val scoredCount: Int,
    val suspiciousCount: Int
)

@Repository
class ScoringResultRepository(
    private val jdbc: JdbcTemplate,
    private val objectMapper: ObjectMapper
) {
    fun findProcessedEvent(eventKey: String): ProcessedScoringEvent? = jdbc.query(
        "SELECT scored_count, suspicious_count FROM scoring_processed_events WHERE event_key=?",
        { rs, _ -> ProcessedScoringEvent(rs.getInt("scored_count"), rs.getInt("suspicious_count")) },
        eventKey
    ).firstOrNull()

    @Transactional
    fun saveRun(
        eventKey: String?,
        jobId: String?,
        datasetId: String,
        featureVersion: Long,
        featureSource: String,
        calculated: List<StoredScoringResult>
    ): Long {
        if (eventKey != null && findProcessedEvent(eventKey) != null) {
            return latestVersion(datasetId) ?: 0
        }
        val version = (latestVersion(datasetId) ?: 0) + 1
        val calculatedAt = calculated.firstOrNull()?.risk?.calculatedAt ?: Instant.now()
        jdbc.update(
            "INSERT INTO scoring_calculations(dataset_id,calculation_version,feature_version,feature_source,calculated_at) VALUES (?,?,?,?,?)",
            datasetId, version, featureVersion, featureSource, Timestamp.from(calculatedAt)
        )
        calculated.forEach { result ->
            val record = result.record
            val risk = result.risk
            jdbc.update(
                """INSERT INTO scoring_results(
                    dataset_id,return_id,calculation_version,order_id,customer_id,support_agent_id,
                    order_amount,refund_amount,product_category,return_reason,evidence_provided,decision,
                    manual_override,decision_time_minutes,source_timestamp,risk_score,risk_level,top_reason,
                    feature_source,relation_features,calculated_at
                ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)""".trimIndent(),
                datasetId, record.returnId, version, record.orderId, record.customerId, record.supportAgentId,
                record.orderAmount, record.refundAmount, record.productCategory, record.returnReason,
                record.evidenceProvided, record.decision, record.manualOverride, record.decisionTimeMinutes,
                record.timestamp, risk.riskScore, risk.riskLevel.name, risk.topReason, featureSource,
                objectMapper.writeValueAsString(result.features), Timestamp.from(risk.calculatedAt)
            )
            risk.reasons.forEachIndexed { index, reason ->
                jdbc.update(
                    "INSERT INTO risk_reasons(dataset_id,return_id,calculation_version,reason_order,reason_type,message,score_impact) VALUES (?,?,?,?,?,?,?)",
                    datasetId, record.returnId, version, index, reason.type, reason.message, reason.scoreImpact
                )
            }
        }
        if (eventKey != null) {
            try {
                jdbc.update(
                    "INSERT INTO scoring_processed_events(event_key,dataset_id,job_id,feature_version,scored_count,suspicious_count,processed_at) VALUES (?,?,?,?,?,?,?)",
                    eventKey, datasetId, jobId, featureVersion, calculated.size,
                    calculated.count { it.risk.riskScore >= 31 }, Timestamp.from(Instant.now())
                )
            } catch (duplicate: DuplicateKeyException) {
                // Another delivery won the idempotency race; transaction rollback is
                // preferable to creating a second calculation version.
                throw duplicate
            }
        }
        return version
    }

    fun latestVersion(datasetId: String): Long? = jdbc.queryForObject(
        "SELECT MAX(calculation_version) FROM scoring_calculations WHERE dataset_id=?",
        Long::class.java,
        datasetId
    )

    fun findLatest(datasetId: String): List<StoredScoringResult> {
        val version = latestVersion(datasetId) ?: return emptyList()
        val reasons = reasonsByReturn(datasetId, version)
        return jdbc.query(
            "SELECT * FROM scoring_results WHERE dataset_id=? AND calculation_version=? ORDER BY return_id",
            { rs, _ -> mapResult(rs, reasons[rs.getString("return_id")].orEmpty()) }, datasetId, version
        )
    }

    fun findLatest(datasetId: String, returnId: String): StoredScoringResult? {
        val version = latestVersion(datasetId) ?: return null
        val reasons = reasonsByReturn(datasetId, version, returnId)[returnId].orEmpty()
        return jdbc.query(
            "SELECT * FROM scoring_results WHERE dataset_id=? AND return_id=? AND calculation_version=?",
            { rs, _ -> mapResult(rs, reasons) }, datasetId, returnId, version
        ).firstOrNull()
    }

    fun findDecision(datasetId: String, returnId: String): InvestigationDecision? = jdbc.query(
        "SELECT * FROM investigation_decisions WHERE dataset_id=? AND return_id=?",
        { rs, _ -> mapDecision(rs) }, datasetId, returnId
    ).firstOrNull()

    fun saveDecision(
        datasetId: String,
        returnId: String,
        request: InvestigationDecisionRequest
    ): InvestigationDecision {
        val existing = findDecision(datasetId, returnId)
        val now = Instant.now()
        if (existing == null) {
            jdbc.update(
                "INSERT INTO investigation_decisions(dataset_id,return_id,action,outcome,note,analyst_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)",
                datasetId, returnId, request.action.name, request.outcome.name, request.note,
                request.analystId, Timestamp.from(now), Timestamp.from(now)
            )
        } else {
            jdbc.update(
                "UPDATE investigation_decisions SET action=?,outcome=?,note=?,analyst_id=?,updated_at=? WHERE dataset_id=? AND return_id=?",
                request.action.name, request.outcome.name, request.note, request.analystId,
                Timestamp.from(now), datasetId, returnId
            )
        }
        return findDecision(datasetId, returnId)!!
    }

    fun decisionsForDataset(datasetId: String): Map<String, InvestigationDecision> = jdbc.query(
        "SELECT * FROM investigation_decisions WHERE dataset_id=?",
        { rs, _ -> mapDecision(rs) }, datasetId
    ).associateBy { it.returnId }

    private fun reasonsByReturn(
        datasetId: String,
        version: Long,
        returnId: String? = null
    ): Map<String, List<RiskReason>> {
        val values = if (returnId == null) {
            jdbc.query(
                "SELECT return_id,reason_type,message,score_impact FROM risk_reasons WHERE dataset_id=? AND calculation_version=? ORDER BY return_id,reason_order",
                { rs, _ -> rs.getString(1) to RiskReason(rs.getString(2), rs.getString(3), rs.getInt(4)) },
                datasetId, version
            )
        } else {
            jdbc.query(
                "SELECT return_id,reason_type,message,score_impact FROM risk_reasons WHERE dataset_id=? AND calculation_version=? AND return_id=? ORDER BY reason_order",
                { rs, _ -> rs.getString(1) to RiskReason(rs.getString(2), rs.getString(3), rs.getInt(4)) },
                datasetId, version, returnId
            )
        }
        return values.groupBy({ it.first }, { it.second })
    }

    private fun mapResult(rs: ResultSet, reasons: List<RiskReason>): StoredScoringResult {
        val datasetId = rs.getString("dataset_id")
        val returnId = rs.getString("return_id")
        val version = rs.getLong("calculation_version")
        val record = RefundApprovalRecord(
            orderId = rs.getString("order_id"), customerId = rs.getString("customer_id"),
            returnId = returnId, supportAgentId = rs.getString("support_agent_id"),
            orderAmount = rs.getDouble("order_amount"), refundAmount = rs.getDouble("refund_amount"),
            productCategory = rs.getString("product_category"), returnReason = rs.getString("return_reason"),
            evidenceProvided = rs.getBoolean("evidence_provided"), decision = rs.getString("decision"),
            manualOverride = rs.getBoolean("manual_override"), decisionTimeMinutes = rs.getInt("decision_time_minutes"),
            timestamp = rs.getString("source_timestamp")
        )
        val featureSource = rs.getString("feature_source")
        val risk = RefundApprovalRiskScore(
            returnId, record.orderId, record.customerId, record.supportAgentId, datasetId,
            rs.getInt("risk_score"), RiskLevel.valueOf(rs.getString("risk_level")),
            rs.getString("top_reason"), reasons, featureSource, version,
            rs.getTimestamp("calculated_at").toInstant()
        )
        val features = objectMapper.readValue(rs.getString("relation_features"), RefundApprovalFeatures::class.java)
        return StoredScoringResult(record, risk, features)
    }

    private fun mapDecision(rs: ResultSet) = InvestigationDecision(
        datasetId = rs.getString("dataset_id"), returnId = rs.getString("return_id"),
        action = InvestigationAction.valueOf(rs.getString("action")),
        outcome = InvestigationOutcome.valueOf(rs.getString("outcome")),
        note = rs.getString("note"), analystId = rs.getString("analyst_id"),
        createdAt = rs.getTimestamp("created_at").toInstant(), updatedAt = rs.getTimestamp("updated_at").toInstant()
    )
}
