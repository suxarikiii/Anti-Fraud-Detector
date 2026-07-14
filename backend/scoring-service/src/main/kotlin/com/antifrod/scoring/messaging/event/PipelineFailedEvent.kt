package com.antifrod.scoring.messaging.event

import java.time.Instant

data class PipelineFailedEvent(
    val datasetId: String,
    val jobId: String? = null,
    val failedStep: String,
    val stage: String = failedStep,
    val errorCode: String,
    val errorMessage: String,
    val eventType: String = "pipeline.failed",
    val timestamp: Instant
)
