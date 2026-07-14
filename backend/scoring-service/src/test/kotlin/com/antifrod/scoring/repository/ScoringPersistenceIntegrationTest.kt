package com.antifrod.scoring.repository

import com.antifrod.scoring.service.ScoringService
import org.junit.jupiter.api.Test
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.jdbc.core.JdbcTemplate
import kotlin.test.assertTrue
import kotlin.test.assertEquals
import java.sql.Timestamp
import java.time.Instant

@SpringBootTest
class ScoringPersistenceIntegrationTest @Autowired constructor(
    private val scoringService: ScoringService,
    private val jdbc: JdbcTemplate
) {
    @Test
    fun `flyway schema stores versioned results reasons and required indexes`() {
        scoringService.recalculateDataset("demo")
        val results = jdbc.queryForObject("SELECT COUNT(*) FROM scoring_results WHERE dataset_id='demo'", Long::class.java)
        val reasons = jdbc.queryForObject("SELECT COUNT(*) FROM risk_reasons WHERE dataset_id='demo'", Long::class.java)
        val indexes = jdbc.queryForList(
            "SELECT index_name FROM information_schema.indexes WHERE index_name LIKE 'idx_%'",
            String::class.java
        ).map { it.lowercase() }.toSet()

        assertTrue((results ?: 0) > 0)
        assertTrue((reasons ?: 0) > 0)
        assertTrue("idx_scoring_results_dataset_risk" in indexes)
        assertTrue("idx_scoring_results_dataset_agent" in indexes)
        assertTrue("idx_decisions_outcome" in indexes)
    }

    @Test
    fun `large export returns every persisted row`() {
        jdbc.update(
            "INSERT INTO scoring_calculations(dataset_id,calculation_version,feature_version,feature_source,calculated_at) VALUES ('large-export',1,1,'RELATIONS_SERVICE',?)",
            Timestamp.from(Instant.now())
        )
        val featuresJson = """{"decision":"APPROVED","evidenceProvided":true,"orderAmount":100.0,"refundAmount":50.0,"refundAmountRatio":0.5,"decisionTimeMinutes":10,"manualOverride":false,"customerReturnCount":1,"agentDecisionCount":1,"agentApprovalRate":0.5,"customerAgentPairCount":1,"clusterSize":1,"strongestRelationType":"NONE","featureSource":"RELATIONS_SERVICE"}"""
        jdbc.update(
            """INSERT INTO scoring_results(
                dataset_id,return_id,calculation_version,order_id,customer_id,support_agent_id,
                order_amount,refund_amount,product_category,return_reason,evidence_provided,decision,
                manual_override,decision_time_minutes,source_timestamp,risk_score,risk_level,top_reason,
                feature_source,relation_features,calculated_at)
                SELECT 'large-export', CONCAT('return_', X), 1, CONCAT('order_', X), CONCAT('customer_', X),
                'agent_bulk', 100.0, 50.0, 'books', 'bulk-test', TRUE, 'APPROVED', FALSE, 10, '',
                10, 'LOW', 'No significant risk factors detected.', 'RELATIONS_SERVICE', ?, CURRENT_TIMESTAMP
                FROM SYSTEM_RANGE(1, 1500) AS bulk(X)""".trimIndent(),
            featuresJson
        )

        val export = scoringService.exportCsv("large-export")
        assertEquals(1501, export.lineSequence().count { it.isNotEmpty() })
        assertTrue(export.contains("return_1500"))
    }
}
