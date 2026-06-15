package com.antifrod.scoring.messaging.event

data class RelationsBuiltEvent(
    val datasetId: String,
    val jobId: String? = null,
    val relationsCount: Int = 0,
    val publishedAt: String? = null
)