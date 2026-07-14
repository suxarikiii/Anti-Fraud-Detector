package com.antifrod.scoring.service

import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.messaging.event.RelationsBuiltEvent
import com.antifrod.scoring.model.InvestigationAction
import com.antifrod.scoring.model.InvestigationDecisionRequest
import com.antifrod.scoring.model.InvestigationOutcome
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.beans.factory.annotation.Autowired
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue
import kotlin.test.assertFalse

@SpringBootTest
class ScoringServiceTest @Autowired constructor(
    private val scoringService: ScoringService
) {

    @Test
    fun `should return suspicious approvals from CSV dataset`() {
        val approvals = scoringService.getSuspiciousApprovals("demo")

        assertTrue(approvals.isNotEmpty())
        assertTrue(approvals.all { it.riskScore >= 31 })
        assertTrue(approvals.zipWithNext().all { (left, right) -> left.riskScore >= right.riskScore })
        assertTrue(approvals.any { it.returnId == "return_3011" })
        assertTrue(approvals.any { it.returnId == "return_3041" })
        assertTrue(approvals.all { it.datasetId == "demo" })
        assertTrue(approvals.all { it.reasons.isNotEmpty() })
    }

    @Test
    fun `should calculate risk for high value refund without evidence`() {
        val risk = scoringService.getReturnRisk("return_3011")

        assertEquals("return_3011", risk.returnId)
        assertEquals("order_1011", risk.orderId)
        assertEquals("customer_400", risk.customerId)
        assertTrue(risk.riskScore >= 81)
        assertEquals(RiskLevel.CRITICAL, risk.riskLevel)

        assertTrue(risk.reasons.any { it.type == "NO_EVIDENCE" })
        assertTrue(risk.reasons.any { it.type == "HIGH_VALUE_REFUND" })
        assertTrue(risk.reasons.any { it.type == "FULL_AMOUNT_REFUND" })
        assertTrue(risk.topReason.isNotBlank())
    }

    @Test
    fun `should calculate critical risk for suspicious cluster case`() {
        val risk = scoringService.getReturnRisk("return_3041")

        assertEquals("return_3041", risk.returnId)
        assertEquals(RiskLevel.CRITICAL, risk.riskLevel)
        assertEquals(100, risk.riskScore)

        assertTrue(risk.reasons.any { it.type == "NO_EVIDENCE" })
        assertTrue(risk.reasons.any { it.type == "FAST_APPROVAL" })
        assertTrue(risk.reasons.any { it.type == "MANUAL_OVERRIDE" })
        assertTrue(risk.reasons.any { it.type == "AGENT_HIGH_APPROVAL_RATE" })
        assertTrue(risk.reasons.any { it.type == "CUSTOMER_FREQUENT_RETURNS" })
        assertTrue(risk.reasons.any { it.type == "REPEATED_AGENT_CUSTOMER_PAIR" })
        assertTrue(risk.reasons.any { it.type == "SUSPICIOUS_CLUSTER" })
    }

    @Test
    fun `should preserve dataset id in dataset aware return risk response`() {
        val risk = scoringService.getReturnRisk("demo", "return_3041")

        assertEquals("demo", risk.datasetId)
        assertEquals("return_3041", risk.returnId)
        assertEquals(RiskLevel.CRITICAL, risk.riskLevel)
    }

    @Test
    fun `should return risk summary for support agent`() {
        val summary = scoringService.getAgentRiskSummary("agent_777")

        assertEquals("agent_777", summary.agentId)
        assertEquals("demo", summary.datasetId)
        assertTrue(summary.totalReturns > 0)
        assertTrue(summary.totalApprovals > 0)
        assertTrue(summary.suspiciousApprovalsCount > 0)
        assertTrue(summary.averageRiskScore > 0.0)
        assertTrue(summary.approvalRate > 0.0)
        assertTrue(summary.topRiskReasons.isNotEmpty())
        assertTrue(summary.topReason.isNotBlank())
    }

    @Test
    fun `should return dataset aware agent risk summary`() {
        val summary = scoringService.getAgentRiskSummary("demo", "agent_777")

        assertEquals("agent_777", summary.agentId)
        assertTrue(summary.suspiciousApprovalsCount > 0)
        assertTrue(summary.averageRiskScore > 0.0)
    }

    @Test
    fun `should return empty summary for unknown support agent`() {
        val summary = scoringService.getAgentRiskSummary("unknown_agent")

        assertEquals("unknown_agent", summary.agentId)
        assertEquals("demo", summary.datasetId)
        assertEquals(0, summary.totalReturns)
        assertEquals(0, summary.totalApprovals)
        assertEquals(0, summary.suspiciousApprovalsCount)
        assertEquals(0.0, summary.averageRiskScore)
        assertEquals(0, summary.highRiskApprovalsCount)
        assertEquals(0, summary.criticalRiskApprovalsCount)
        assertEquals(0.0, summary.approvalRate)
        assertTrue(summary.topRiskReasons.isEmpty())
        assertEquals("No refund approvals found for this support agent", summary.topReason)
    }

    @Test
    fun `should return recalculate response`() {
        val response = scoringService.recalculateDataset("demo")

        assertEquals("demo", response.datasetId)
        assertTrue(response.status.contains("RECALCULATED"))
        assertTrue(response.status.contains("VERSION"))
    }

    @Test
    fun `should process relations built event`() {
        val result = scoringService.processRelationsBuilt("demo")

        assertTrue(result.suspiciousApprovalsCount > 0)
    }

    @Test
    fun `should include calculated timestamp in risk response`() {
        val risk = scoringService.getReturnRisk("return_3006")

        assertNotNull(risk.calculatedAt)
    }

    @Test
    fun `should include fast approval reason`() {
        val risk = scoringService.getReturnRisk("return_3016")

        assertTrue(risk.reasons.any { it.type == "FAST_APPROVAL" })
    }

    @Test
    fun `should return refund approval details with risk data`() {
        val details = scoringService.getReturnDetails("demo", "return_3041")

        assertEquals("return_3041", details.returnId)
        assertEquals("demo", details.datasetId)
        assertEquals("order_1041", details.orderId)
        assertEquals("customer_999", details.customerId)
        assertEquals("agent_999", details.supportAgentId)

        assertTrue(details.orderAmount > 0.0)
        assertTrue(details.refundAmount > 0.0)
        assertEquals("APPROVED", details.decision)
        assertEquals(RiskLevel.CRITICAL, details.riskLevel)
        assertEquals(100, details.riskScore)
        assertTrue(details.reasons.isNotEmpty())
        assertEquals("DEMO_CSV", details.relationFeatures.featureSource)
        assertTrue(details.relationFeatures.customerReturnCount >= 5)
    }

    @Test
    fun `unknown UUID dataset never receives demo records`() {
        kotlin.test.assertFailsWith<ScoringDependencyException> {
            scoringService.getReturnDetails("123e4567-e89b-12d3-a456-426614174000", "return_3041")
        }
    }

    @Test
    fun `duplicate relations event is idempotent`() {
        val event = RelationsBuiltEvent(datasetId = "demo", jobId = "idempotency-test-job")
        val first = scoringService.processRelationsBuilt(event)
        val second = scoringService.processRelationsBuilt(event)

        assertFalse(first.duplicate)
        assertTrue(second.duplicate)
        assertEquals(first.scoredApprovalsCount, second.scoredApprovalsCount)
        assertEquals(first.suspiciousApprovalsCount, second.suspiciousApprovalsCount)
    }

    @Test
    fun `recalculation increments persisted calculation version`() {
        val before = scoringService.getReturnRisk("demo", "return_3002").calculationVersion
        scoringService.recalculateDataset("demo")
        val after = scoringService.getReturnRisk("demo", "return_3002").calculationVersion

        assertTrue(after > before)
    }

    @Test
    fun `decision survives recalculation and CSV export escapes UTF-8 note`() {
        val request = InvestigationDecisionRequest(
            action = InvestigationAction.ESCALATE,
            outcome = InvestigationOutcome.NEEDS_MORE_INFO,
            note = "Проверить чек, \"важно\"",
            analystId = "analyst-1"
        )
        scoringService.saveDecision("demo", "return_3003", request)
        scoringService.recalculateDataset("demo")

        val stored = scoringService.getDecision("demo", "return_3003")
        val export = scoringService.exportCsv("demo", outcome = InvestigationOutcome.NEEDS_MORE_INFO)
        assertEquals(request.note, stored.note)
        assertTrue(export.startsWith("\uFEFF"))
        assertTrue(export.contains("Проверить чек, \"\"важно\"\""))
        assertTrue(export.contains("return_3003"))
    }

    @Test
    fun `terminal analyst outcome rejects invalid transition`() {
        scoringService.saveDecision(
            "demo", "return_3004",
            InvestigationDecisionRequest(InvestigationAction.REVIEW, InvestigationOutcome.CONFIRMED_FRAUD)
        )
        kotlin.test.assertFailsWith<ScoringValidationException> {
            scoringService.saveDecision(
                "demo", "return_3004",
                InvestigationDecisionRequest(InvestigationAction.REVIEW, InvestigationOutcome.OPEN)
            )
        }
    }

    @Test
    fun `empty filtered export still returns a valid header`() {
        val export = scoringService.exportCsv("demo", agent = "agent-does-not-exist")
        assertTrue(export.startsWith("\uFEFF\"datasetId\""))
        assertEquals(2, export.split("\r\n").size)
    }
}
