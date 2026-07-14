package com.antifrod.scoring.model

import jakarta.validation.constraints.Size
import java.time.Instant

enum class InvestigationAction {
    REVIEW, ESCALATE, APPROVE_REFUND, REJECT_REFUND, FREEZE_ACCOUNT
}

enum class InvestigationOutcome {
    OPEN, NEEDS_MORE_INFO, CONFIRMED_FRAUD, FALSE_POSITIVE, RESOLVED
}

data class InvestigationDecisionRequest(
    val action: InvestigationAction,
    val outcome: InvestigationOutcome,
    @field:Size(max = 10_000) val note: String = "",
    @field:Size(max = 255) val analystId: String = "anonymous"
)

data class InvestigationDecision(
    val datasetId: String,
    val returnId: String,
    val action: InvestigationAction,
    val outcome: InvestigationOutcome,
    val note: String,
    val analystId: String,
    val createdAt: Instant,
    val updatedAt: Instant
)
