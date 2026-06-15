package com.antifrod.scoring.messaging.event

import java.time.Instant

data class PipelineFailedEvent(
    val datasetId: String,
    val jobId: String? = null,
    val failedStage: String,
    val message: String,
    val eventType: String = "pipeline.failed",
    val publishedAt: Instant
)