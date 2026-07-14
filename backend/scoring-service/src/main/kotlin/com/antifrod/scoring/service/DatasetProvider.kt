package com.antifrod.scoring.service

import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RefundApprovalRecord

data class DatasetSnapshot(
    val datasetId: String,
    val featureVersion: Long,
    val featureSource: String,
    val records: List<RefundApprovalRecord>,
    val features: Map<String, RefundApprovalFeatures>
)

interface DatasetProvider {
    fun load(datasetId: String): DatasetSnapshot
}

class ScoringDependencyException(
    message: String,
    val errorCode: String,
    cause: Throwable? = null
) : RuntimeException(message, cause)
