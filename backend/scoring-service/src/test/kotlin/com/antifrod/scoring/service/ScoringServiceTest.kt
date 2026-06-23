package com.antifrod.scoring.service

import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.repository.RefundDatasetRepository
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class ScoringServiceTest {

    private val scoringService = ScoringService(
        refundDatasetRepository = RefundDatasetRepository()
    )

    @Test
    fun `should return suspicious approvals from CSV dataset`() {
        val approvals = scoringService.getSuspiciousApprovals("demo")

        assertTrue(approvals.isNotEmpty())
        assertTrue(approvals.all { it.riskScore >= 31 })
        assertTrue(approvals.any { it.returnId == "return_3011" })
        assertTrue(approvals.any { it.returnId == "return_3041" })
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
    }

    @Test
    fun `should calculate critical risk for suspicious cluster case`() {
        val risk = scoringService.getReturnRisk("return_3041")

        assertEquals("return_3041", risk.returnId)
        assertEquals(RiskLevel.CRITICAL, risk.riskLevel)
        assertEquals(100, risk.riskScore)

        assertTrue(risk.reasons.any { it.type == "NO_EVIDENCE" })
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
        assertTrue(summary.suspiciousApprovalsCount > 0)
        assertTrue(summary.averageRiskScore > 0.0)
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
        assertEquals(0, summary.suspiciousApprovalsCount)
        assertEquals(0.0, summary.averageRiskScore)
        assertEquals(0, summary.highRiskApprovalsCount)
        assertEquals(0, summary.criticalRiskApprovalsCount)
        assertEquals("No refund approvals found for this support agent", summary.topReason)
    }

    @Test
    fun `should return recalculate response`() {
        val response = scoringService.recalculateDataset("demo")

        assertEquals("demo", response.datasetId)
        assertTrue(response.status.contains("RECALCULATION_STARTED"))
        assertTrue(response.status.contains("SUSPICIOUS_APPROVALS"))
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
    }
}
