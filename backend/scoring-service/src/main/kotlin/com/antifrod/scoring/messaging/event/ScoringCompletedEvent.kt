package com.antifrod.scoring.messaging.event

import java.time.Instant

data class ScoringCompletedEvent(
    val datasetId: String,
    val jobId: String? = null,
    val scoredApprovalsCount: Int,
    val suspiciousApprovalsCount: Int,
    val status: String = "COMPLETED",
    val eventType: String = "refund.scoring.completed",
    val publishedAt: Instant
)