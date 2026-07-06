package com.antifrod.scoring.service

import com.antifrod.scoring.model.RefundApprovalFeatures
import org.springframework.stereotype.Component
import java.util.Locale
import kotlin.math.roundToInt

@Component
class ExplanationBuilder {

    fun message(type: String, features: RefundApprovalFeatures): String {
        return when (type) {
            "NO_EVIDENCE" ->
                "Refund was approved without attached evidence, so the analyst cannot verify the customer's claim from this record."
            "HIGH_VALUE_REFUND" ->
                "Refund amount is ${money(features.refundAmount)}. This is above the ${money(HIGH_VALUE_REFUND_THRESHOLD)} high-value threshold and should be checked before payout."
            "FULL_AMOUNT_REFUND" ->
                "Refund covers ${percent(features.refundAmountRatio)} of the original order amount, so the customer effectively received a full refund."
            "FAST_APPROVAL" ->
                "Decision was approved in ${features.decisionTimeMinutes} minutes. This is at or below the 5-minute review threshold and may indicate a skipped check."
            "MANUAL_OVERRIDE" ->
                "Manual override was used, so the approval bypassed the standard decision path."
            "AGENT_HIGH_APPROVAL_RATE" ->
                "This support agent approved ${percent(features.agentApprovalRate)} of ${features.agentDecisionCount} refund decisions in the dataset; compare this with team norms before accepting the case."
            "CUSTOMER_FREQUENT_RETURNS" ->
                "This customer has ${features.customerReturnCount} return requests in the dataset, which may indicate repeat refund behavior."
            "REPEATED_AGENT_CUSTOMER_PAIR" ->
                "This same agent-customer pair appears in ${features.customerAgentPairCount} return requests, so repeated approvals should be reviewed."
            "SUSPICIOUS_CLUSTER" ->
                "This return is part of a relation cluster of ${features.clusterSize} connected returns; investigate linked requests before closing the case."
            else -> "No significant risk factors detected."
        }
    }

    private fun percent(value: Double): String {
        return "${(value * 100).roundToInt()}%"
    }

    private fun money(value: Double): String {
        return "$" + String.format(Locale.US, "%.2f", value)
    }

    private companion object {
        const val HIGH_VALUE_REFUND_THRESHOLD = 500.0
    }
}
