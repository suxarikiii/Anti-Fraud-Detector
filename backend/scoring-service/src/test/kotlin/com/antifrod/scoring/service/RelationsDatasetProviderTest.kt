package com.antifrod.scoring.service

import org.junit.jupiter.api.Test
import org.springframework.http.HttpMethod
import org.springframework.http.MediaType
import org.springframework.test.web.client.MockRestServiceServer
import org.springframework.test.web.client.ExpectedCount.once
import org.springframework.test.web.client.match.MockRestRequestMatchers.method
import org.springframework.test.web.client.match.MockRestRequestMatchers.requestTo
import org.springframework.test.web.client.response.MockRestResponseCreators.withResourceNotFound
import org.springframework.test.web.client.response.MockRestResponseCreators.withSuccess
import org.springframework.web.client.RestClient
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotEquals

class RelationsDatasetProviderTest {
    @Test
    fun `two dataset ids use their own records and relation features`() {
        val builder = RestClient.builder()
        val server = MockRestServiceServer.bindTo(builder).build()
        val provider = RelationsDatasetProvider(builder, "http://relations")
        server.expect(once(), requestTo("http://relations/api/relations/datasets/dataset-a/scoring-inputs"))
            .andExpect(method(HttpMethod.GET))
            .andRespond(withSuccess(payload("dataset-a", "return-a", 2, 0.5), MediaType.APPLICATION_JSON))
        server.expect(once(), requestTo("http://relations/api/relations/datasets/dataset-b/scoring-inputs"))
            .andRespond(withSuccess(payload("dataset-b", "return-b", 8, 1.0), MediaType.APPLICATION_JSON))

        val first = provider.load("dataset-a")
        val second = provider.load("dataset-b")

        assertEquals(setOf("return-a"), first.records.map { it.returnId }.toSet())
        assertEquals(setOf("return-b"), second.records.map { it.returnId }.toSet())
        assertNotEquals(first.features["return-a"]?.customerReturnCount, second.features["return-b"]?.customerReturnCount)
        assertEquals("RELATIONS_SERVICE", second.featureSource)
        val engine = RiskRuleEngine(ExplanationBuilder())
        val firstScore = engine.calculateScore(engine.calculateReasons(first.features.getValue("return-a")))
        val secondScore = engine.calculateScore(engine.calculateReasons(second.features.getValue("return-b")))
        assertNotEquals(firstScore, secondScore, "relation features must affect scoring rules")
        server.verify()
    }

    @Test
    fun `relations 404 stays unknown dataset and never becomes demo`() {
        val builder = RestClient.builder()
        val server = MockRestServiceServer.bindTo(builder).build()
        val provider = RelationsDatasetProvider(builder, "http://relations")
        server.expect(requestTo("http://relations/api/relations/datasets/missing/scoring-inputs"))
            .andRespond(withResourceNotFound())

        assertFailsWith<ScoringNotFoundException> { provider.load("missing") }
        server.verify()
    }

    private fun payload(datasetId: String, returnId: String, customerReturns: Int, approvalRate: Double) = """
        {
          "datasetId":"$datasetId","schemaVersion":"refund-normalized.v1","featureVersion":7,
          "calculatedAt":"2026-07-14T12:00:00Z",
          "records":[{
            "datasetId":"$datasetId","returnId":"$returnId","customerId":"customer","orderId":"order",
            "supportAgentId":"agent","productCategory":"books","returnReason":"damaged",
            "decisionStatus":"APPROVED","refundAmount":90.0,"orderAmount":100.0,
            "evidenceProvided":true,"manualOverride":false,"decisionTimeMinutes":10,"timestamp":"2026-07-14T10:00:00Z"
          }],
          "features":[{
            "returnId":"$returnId","features":{"customerReturnCount":$customerReturns,
            "agentDecisionCount":10,"agentApprovalRate":$approvalRate,"customerAgentPairCount":1,
            "refundAmountRatio":0.9,"clusterSize":$customerReturns,"strongestRelationType":"CUSTOMER_FREQUENT_RETURNS"}
          }]
        }
    """.trimIndent()
}
