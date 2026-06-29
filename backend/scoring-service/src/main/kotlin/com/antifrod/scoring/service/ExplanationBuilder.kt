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
                "Refund was approved without evidence for this return request."
            "HIGH_VALUE_REFUND" ->
                "Refund amount is ${money(features.refundAmount)} (${percent(features.refundAmountRatio)} of the order amount), which is above the high-value threshold."
            "FULL_AMOUNT_REFUND" ->
                "Refund amount is ${percent(features.refundAmountRatio)} of the original order amount, which is close to a full refund."
            "FAST_APPROVAL" ->
                "Refund was approved in ${features.decisionTimeMinutes} minutes, which is faster than the normal review threshold."
            "MANUAL_OVERRIDE" ->
                "Manual override was used for this refund approval."
            "AGENT_HIGH_APPROVAL_RATE" ->
                "Support agent approved ${percent(features.agentApprovalRate)} of ${features.agentDecisionCount} refund decisions in this dataset."
            "CUSTOMER_FREQUENT_RETURNS" ->
                "Customer has ${features.customerReturnCount} return requests in this dataset, which is above the suspicious threshold."
            "REPEATED_AGENT_CUSTOMER_PAIR" ->
                "The same support agent handled ${features.customerAgentPairCount} return requests for this customer."
            "SUSPICIOUS_CLUSTER" ->
                "Approval belongs to a suspicious relation cluster of size ${features.clusterSize}."
            else -> "No significant risk factors detected."
        }
    }

    private fun percent(value: Double): String {
        return "${(value * 100).roundToInt()}%"
    }

    private fun money(value: Double): String {
        return "$" + String.format(Locale.US, "%.2f", value)
    }
}
