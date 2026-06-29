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
