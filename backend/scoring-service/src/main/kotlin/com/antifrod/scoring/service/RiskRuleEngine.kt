package com.antifrod.scoring.service

import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.model.RiskReason
import org.springframework.stereotype.Component

@Component
class RiskRuleEngine(
    private val explanationBuilder: ExplanationBuilder
) {

    fun calculateReasons(features: RefundApprovalFeatures): List<RiskReason> {
        return RULES
            .filter { rule -> rule.applies(features) }
            .map { rule ->
                RiskReason(
                    type = rule.type,
                    message = explanationBuilder.message(rule.type, features),
                    scoreImpact = rule.scoreImpact
                )
            }
    }

    fun calculateScore(reasons: List<RiskReason>): Int {
        return reasons.sumOf { it.scoreImpact }.coerceIn(0, 100)
    }

    fun resolveRiskLevel(score: Int): RiskLevel {
        val cappedScore = score.coerceIn(0, 100)
        return when {
            cappedScore >= 81 -> RiskLevel.CRITICAL
            cappedScore >= 61 -> RiskLevel.HIGH
            cappedScore >= 31 -> RiskLevel.MEDIUM
            else -> RiskLevel.LOW
        }
    }

    fun topReason(reasons: List<RiskReason>): String {
        return reasons.firstOrNull()?.message ?: "No significant risk factors detected."
    }

    fun reasonPriority(type: String): Int {
        return RULES.indexOfFirst { it.type == type }.let { index ->
            if (index == -1) Int.MAX_VALUE else index
        }
    }

    private data class RiskRule(
        val type: String,
        val scoreImpact: Int,
        val applies: (RefundApprovalFeatures) -> Boolean
    )

    private companion object {
        const val APPROVED = "APPROVED"

        val RULES = listOf(
            RiskRule("NO_EVIDENCE", 25) { features ->
                features.decision == APPROVED && !features.evidenceProvided
            },
            RiskRule("HIGH_VALUE_REFUND", 20) { features ->
                features.refundAmount >= 500.0
            },
            RiskRule("FULL_AMOUNT_REFUND", 15) { features ->
                features.refundAmountRatio >= 0.95
            },
            RiskRule("FAST_APPROVAL", 15) { features ->
                features.decision == APPROVED && features.decisionTimeMinutes <= 5
            },
            RiskRule("MANUAL_OVERRIDE", 20) { features ->
                features.manualOverride
            },
            RiskRule("AGENT_HIGH_APPROVAL_RATE", 30) { features ->
                features.agentDecisionCount >= 5 && features.agentApprovalRate > 0.85
            },
            RiskRule("CUSTOMER_FREQUENT_RETURNS", 20) { features ->
                features.customerReturnCount >= 5
            },
            RiskRule("REPEATED_AGENT_CUSTOMER_PAIR", 25) { features ->
                features.customerAgentPairCount >= 3
            },
            RiskRule("SUSPICIOUS_CLUSTER", 25) { features ->
                features.clusterSize >= 5
            }
        )
    }
}
