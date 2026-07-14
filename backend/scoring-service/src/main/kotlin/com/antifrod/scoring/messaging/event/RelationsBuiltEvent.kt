package com.antifrod.scoring.messaging.event

data class RelationsBuiltEvent(
    val datasetId: String,
    val jobId: String? = null,
    val recordsPath: String? = null,
    val recordsCount: Int = 0,
    val relationsCount: Int = 0,
    val featuresCount: Int = 0,
    val schemaVersion: String? = null,
    val featureVersion: Long = 0,
    val featuresReady: Boolean = true,
    val publishedAt: String? = null,
    val timestamp: String? = null
) {
    fun idempotencyKey(): String = jobId?.takeIf { it.isNotBlank() }?.let { "$datasetId:$it:$featureVersion" }
        ?: "$datasetId:feature:$featureVersion"
}
