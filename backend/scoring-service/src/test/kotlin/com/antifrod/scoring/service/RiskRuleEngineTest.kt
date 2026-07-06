package com.antifrod.scoring.service

import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.model.RiskReason
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class RiskRuleEngineTest {

    private val riskRuleEngine = RiskRuleEngine(ExplanationBuilder())

    @Test
    fun `should emit all supported reason types deterministically`() {
        val reasons = riskRuleEngine.calculateReasons(
            features(
                evidenceProvided = false,
                refundAmount = 1_200.0,
                orderAmount = 1_200.0,
                refundAmountRatio = 1.0,
                decisionTimeMinutes = 2,
                manualOverride = true,
                customerReturnCount = 6,
                agentDecisionCount = 10,
                agentApprovalRate = 0.9,
                customerAgentPairCount = 4,
                clusterSize = 6
            )
        )

        assertEquals(
            listOf(
                "NO_EVIDENCE",
                "HIGH_VALUE_REFUND",
                "FULL_AMOUNT_REFUND",
                "FAST_APPROVAL",
                "MANUAL_OVERRIDE",
                "AGENT_HIGH_APPROVAL_RATE",
                "CUSTOMER_FREQUENT_RETURNS",
                "REPEATED_AGENT_CUSTOMER_PAIR",
                "SUSPICIOUS_CLUSTER"
            ),
            reasons.map { it.type }
        )
        assertTrue(reasons.all { it.message.isNotBlank() })
        assertEquals(100, riskRuleEngine.calculateScore(reasons))
    }

    @Test
    fun `should calculate each individual rule reason`() {
        assertReason("NO_EVIDENCE", features(evidenceProvided = false))
        assertReason("HIGH_VALUE_REFUND", features(refundAmount = 500.0))
        assertReason("FULL_AMOUNT_REFUND", features(refundAmount = 95.0, orderAmount = 100.0, refundAmountRatio = 0.95))
        assertReason("FAST_APPROVAL", features(decisionTimeMinutes = 5))
        assertReason("MANUAL_OVERRIDE", features(manualOverride = true))
        assertReason("AGENT_HIGH_APPROVAL_RATE", features(agentDecisionCount = 5, agentApprovalRate = 0.86))
        assertReason("CUSTOMER_FREQUENT_RETURNS", features(customerReturnCount = 5, clusterSize = 4))
        assertReason("REPEATED_AGENT_CUSTOMER_PAIR", features(customerAgentPairCount = 3))
        assertReason("SUSPICIOUS_CLUSTER", features(clusterSize = 5))
    }

    @Test
    fun `should build analyst readable explanations with business context`() {
        val reasons = riskRuleEngine.calculateReasons(
            features(
                evidenceProvided = false,
                refundAmount = 1_019.25,
                orderAmount = 1_168.27,
                refundAmountRatio = 1_019.25 / 1_168.27,
                decisionTimeMinutes = 4,
                manualOverride = true,
                customerReturnCount = 5,
                agentDecisionCount = 5,
                agentApprovalRate = 1.0,
                customerAgentPairCount = 5,
                clusterSize = 5
            )
        )

        assertMessageContains(reasons, "NO_EVIDENCE", "analyst cannot verify")
        assertMessageContains(reasons, "HIGH_VALUE_REFUND", "$500.00 high-value threshold")
        assertMessageContains(reasons, "FAST_APPROVAL", "5-minute review threshold")
        assertMessageContains(reasons, "MANUAL_OVERRIDE", "standard decision path")
        assertMessageContains(reasons, "AGENT_HIGH_APPROVAL_RATE", "compare this with team norms")
        assertMessageContains(reasons, "CUSTOMER_FREQUENT_RETURNS", "repeat refund behavior")
        assertMessageContains(reasons, "REPEATED_AGENT_CUSTOMER_PAIR", "repeated approvals should be reviewed")
        assertMessageContains(reasons, "SUSPICIOUS_CLUSTER", "investigate linked requests")
    }

    @Test
    fun `should resolve required risk level boundaries`() {
        mapOf(
            0 to RiskLevel.LOW,
            30 to RiskLevel.LOW,
            31 to RiskLevel.MEDIUM,
            60 to RiskLevel.MEDIUM,
            61 to RiskLevel.HIGH,
            80 to RiskLevel.HIGH,
            81 to RiskLevel.CRITICAL,
            100 to RiskLevel.CRITICAL,
            120 to RiskLevel.CRITICAL
        ).forEach { (score, expectedLevel) ->
            assertEquals(expectedLevel, riskRuleEngine.resolveRiskLevel(score), "score=$score")
        }
    }

    @Test
    fun `should cap additive score at 100`() {
        val reasons = listOf(
            RiskReason("A", "first", 80),
            RiskReason("B", "second", 50)
        )

        assertEquals(100, riskRuleEngine.calculateScore(reasons))
    }

    private fun assertReason(type: String, features: RefundApprovalFeatures) {
        assertTrue(
            riskRuleEngine.calculateReasons(features).any { it.type == type },
            "Expected reason $type"
        )
    }

    private fun assertMessageContains(reasons: List<RiskReason>, type: String, expectedText: String) {
        val reason = reasons.first { it.type == type }
        assertTrue(
            reason.message.contains(expectedText),
            "Expected $type message to contain '$expectedText' but was '${reason.message}'"
        )
    }

    private fun features(
        decision: String = "APPROVED",
        evidenceProvided: Boolean = true,
        orderAmount: Double = 100.0,
        refundAmount: Double = 10.0,
        refundAmountRatio: Double = refundAmount / orderAmount,
        decisionTimeMinutes: Int = 30,
        manualOverride: Boolean = false,
        customerReturnCount: Int = 1,
        agentDecisionCount: Int = 1,
        agentApprovalRate: Double = 0.5,
        customerAgentPairCount: Int = 1,
        clusterSize: Int = 1,
        strongestRelationType: String = "NONE",
        featureSource: String = "TEST"
    ): RefundApprovalFeatures {
        return RefundApprovalFeatures(
            decision = decision,
            evidenceProvided = evidenceProvided,
            orderAmount = orderAmount,
            refundAmount = refundAmount,
            refundAmountRatio = refundAmountRatio,
            decisionTimeMinutes = decisionTimeMinutes,
            manualOverride = manualOverride,
            customerReturnCount = customerReturnCount,
            agentDecisionCount = agentDecisionCount,
            agentApprovalRate = agentApprovalRate,
            customerAgentPairCount = customerAgentPairCount,
            clusterSize = clusterSize,
            strongestRelationType = strongestRelationType,
            featureSource = featureSource
        )
    }
}
