package com.antifrod.scoring.service

import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RefundApprovalRecord
import org.springframework.beans.factory.annotation.Value
import org.springframework.http.HttpStatusCode
import org.springframework.stereotype.Component
import org.springframework.web.client.RestClient
import org.springframework.web.client.RestClientException
import java.time.Instant

@Component
class RelationsDatasetProvider(
    restClientBuilder: RestClient.Builder,
    @Value("\${app.relations.base-url}") baseUrl: String
) : DatasetProvider {
    private val client = restClientBuilder.baseUrl(baseUrl).build()

    override fun load(datasetId: String): DatasetSnapshot {
        val response = try {
            client.get()
                .uri("/api/relations/datasets/{datasetId}/scoring-inputs", datasetId)
                .retrieve()
                .onStatus({ status -> status.value() == 404 }) { _, _ ->
                    throw ScoringNotFoundException("Dataset was not found: $datasetId")
                }
                .onStatus(HttpStatusCode::isError) { _, response ->
                    throw ScoringDependencyException(
                        "Relations service rejected dataset $datasetId with HTTP ${response.statusCode.value()}",
                        "RELATIONS_ERROR"
                    )
                }
                .body(RelationsScoringInputs::class.java)
        } catch (exception: ScoringNotFoundException) {
            throw exception
        } catch (exception: ScoringDependencyException) {
            throw exception
        } catch (exception: RestClientException) {
            throw ScoringDependencyException(
                "Relations service is unavailable for dataset $datasetId",
                "RELATIONS_UNAVAILABLE",
                exception
            )
        } ?: throw ScoringDependencyException(
            "Relations service returned an empty response for dataset $datasetId",
            "RELATIONS_EMPTY_RESPONSE"
        )

        if (response.datasetId != datasetId || response.records.isEmpty()) {
            throw ScoringDependencyException(
                "Relations response does not match dataset $datasetId",
                "RELATIONS_CONTRACT_VIOLATION"
            )
        }
        val byReturn = response.features.associateBy { it.returnId }
        val records = response.records.map { record ->
            if (record.datasetId != datasetId) {
                throw ScoringDependencyException(
                    "Relations response contains a record from another dataset",
                    "RELATIONS_CONTRACT_VIOLATION"
                )
            }
            RefundApprovalRecord(
                orderId = record.orderId,
                customerId = record.customerId,
                returnId = record.returnId,
                supportAgentId = record.supportAgentId,
                orderAmount = record.orderAmount,
                refundAmount = record.refundAmount,
                productCategory = record.productCategory,
                returnReason = record.returnReason,
                evidenceProvided = record.evidenceProvided,
                decision = record.decisionStatus.uppercase(),
                manualOverride = record.manualOverride,
                decisionTimeMinutes = record.decisionTimeMinutes,
                timestamp = record.timestamp
            )
        }
        val features = records.associate { record ->
            val relation = byReturn[record.returnId]
                ?: throw ScoringDependencyException(
                    "Relations features are missing for return ${record.returnId}",
                    "RELATIONS_FEATURES_MISSING"
                )
            record.returnId to RefundApprovalFeatures(
                decision = record.decision,
                evidenceProvided = record.evidenceProvided,
                orderAmount = record.orderAmount,
                refundAmount = record.refundAmount,
                refundAmountRatio = relation.features.refundAmountRatio,
                decisionTimeMinutes = record.decisionTimeMinutes,
                manualOverride = record.manualOverride,
                customerReturnCount = relation.features.customerReturnCount,
                agentDecisionCount = relation.features.agentDecisionCount,
                agentApprovalRate = relation.features.agentApprovalRate,
                customerAgentPairCount = relation.features.customerAgentPairCount,
                clusterSize = relation.features.clusterSize,
                strongestRelationType = relation.features.strongestRelationType,
                featureSource = "RELATIONS_SERVICE"
            )
        }
        return DatasetSnapshot(datasetId, response.featureVersion, "RELATIONS_SERVICE", records, features)
    }
}

data class RelationsScoringInputs(
    val datasetId: String = "",
    val schemaVersion: String = "",
    val featureVersion: Long = 0,
    val calculatedAt: Instant? = null,
    val records: List<RelationsRecord> = emptyList(),
    val features: List<RelationsFeatureEnvelope> = emptyList()
)

data class RelationsRecord(
    val datasetId: String = "",
    val returnId: String = "",
    val customerId: String = "",
    val orderId: String = "",
    val supportAgentId: String = "",
    val productCategory: String = "",
    val returnReason: String = "",
    val decisionStatus: String = "",
    val refundAmount: Double = 0.0,
    val orderAmount: Double = 0.0,
    val evidenceProvided: Boolean = false,
    val manualOverride: Boolean = false,
    val decisionTimeMinutes: Int = 0,
    val timestamp: String = ""
)

data class RelationsFeatureEnvelope(
    val returnId: String = "",
    val features: RelationsFeatures = RelationsFeatures()
)

data class RelationsFeatures(
    val customerReturnCount: Int = 0,
    val agentDecisionCount: Int = 0,
    val agentApprovalRate: Double = 0.0,
    val customerAgentPairCount: Int = 0,
    val refundAmountRatio: Double = 0.0,
    val clusterSize: Int = 0,
    val strongestRelationType: String = "NONE"
)
