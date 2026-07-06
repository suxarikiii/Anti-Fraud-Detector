package com.antifrod.scoring.controller

import org.hamcrest.Matchers.greaterThan
import org.hamcrest.Matchers.greaterThanOrEqualTo
import org.hamcrest.Matchers.hasItem
import org.junit.jupiter.api.Test
import org.springframework.amqp.rabbit.core.RabbitTemplate
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.boot.test.mock.mockito.MockBean
import org.springframework.test.web.servlet.MockMvc
import org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get
import org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath
import org.springframework.test.web.servlet.result.MockMvcResultMatchers.status

@SpringBootTest(
    properties = [
        "spring.rabbitmq.dynamic=false",
        "spring.rabbitmq.listener.simple.auto-startup=false",
        "spring.rabbitmq.listener.direct.auto-startup=false"
    ]
)
@AutoConfigureMockMvc
class ScoringControllerIntegrationTest @Autowired constructor(
    private val mockMvc: MockMvc
) {

    @MockBean
    lateinit var rabbitTemplate: RabbitTemplate

    @Test
    fun `should return scoring health status`() {
        mockMvc.perform(get("/api/scoring/health"))
            .andExpect(status().isOk)
            .andExpect(jsonPath("$.status").value("UP"))
            .andExpect(jsonPath("$.service").value("scoring-service"))
    }

    @Test
    fun `should return suspicious approvals from CSV through HTTP endpoint`() {
        mockMvc.perform(get("/api/scoring/datasets/demo/suspicious-approvals"))
            .andExpect(status().isOk)
            .andExpect(jsonPath("$").isArray)
            .andExpect(jsonPath("$.length()", greaterThan(0)))
            .andExpect(jsonPath("$[*].returnId", hasItem("return_3041")))
            .andExpect(jsonPath("$[0].datasetId").value("demo"))
            .andExpect(jsonPath("$[0].orderAmount").exists())
            .andExpect(jsonPath("$[0].riskScore").exists())
            .andExpect(jsonPath("$[0].riskLevel").exists())
            .andExpect(jsonPath("$[0].topReason").exists())
            .andExpect(jsonPath("$[0].reasons").isArray)
            .andExpect(jsonPath("$[0].calculatedAt").exists())
    }

    @Test
    fun `should return critical risk for return approval through HTTP endpoint`() {
        mockMvc.perform(get("/api/scoring/returns/return_3041/risk"))
            .andExpect(status().isOk)
            .andExpect(jsonPath("$.returnId").value("return_3041"))
            .andExpect(jsonPath("$.orderId").value("order_1041"))
            .andExpect(jsonPath("$.customerId").value("customer_999"))
            .andExpect(jsonPath("$.supportAgentId").value("agent_999"))
            .andExpect(jsonPath("$.datasetId").value("demo"))
            .andExpect(jsonPath("$.riskScore").value(100))
            .andExpect(jsonPath("$.riskLevel").value("CRITICAL"))
            .andExpect(jsonPath("$.reasons[*].type", hasItem("NO_EVIDENCE")))
            .andExpect(jsonPath("$.reasons[*].type", hasItem("FAST_APPROVAL")))
            .andExpect(jsonPath("$.reasons[*].type", hasItem("MANUAL_OVERRIDE")))
            .andExpect(jsonPath("$.reasons[*].type", hasItem("CUSTOMER_FREQUENT_RETURNS")))
            .andExpect(jsonPath("$.reasons[*].type", hasItem("REPEATED_AGENT_CUSTOMER_PAIR")))
            .andExpect(jsonPath("$.reasons[*].type", hasItem("SUSPICIOUS_CLUSTER")))
    }

    @Test
    fun `should return dataset aware return risk through HTTP endpoint`() {
        mockMvc.perform(get("/api/scoring/datasets/demo/returns/return_3041/risk"))
            .andExpect(status().isOk)
            .andExpect(jsonPath("$.returnId").value("return_3041"))
            .andExpect(jsonPath("$.datasetId").value("demo"))
            .andExpect(jsonPath("$.riskScore").value(100))
            .andExpect(jsonPath("$.riskLevel").value("CRITICAL"))
    }

    @Test
    fun `should return refund approval details with risk data through HTTP endpoint`() {
        mockMvc.perform(get("/api/scoring/datasets/demo/returns/return_3041/details"))
            .andExpect(status().isOk)
            .andExpect(jsonPath("$.returnId").value("return_3041"))
            .andExpect(jsonPath("$.orderId").value("order_1041"))
            .andExpect(jsonPath("$.customerId").value("customer_999"))
            .andExpect(jsonPath("$.supportAgentId").value("agent_999"))
            .andExpect(jsonPath("$.datasetId").value("demo"))
            .andExpect(jsonPath("$.orderAmount").value(1168.27))
            .andExpect(jsonPath("$.refundAmount").value(1019.25))
            .andExpect(jsonPath("$.productCategory").value("electronics"))
            .andExpect(jsonPath("$.returnReason").value("item_not_as_described"))
            .andExpect(jsonPath("$.evidenceProvided").value(false))
            .andExpect(jsonPath("$.decision").value("APPROVED"))
            .andExpect(jsonPath("$.manualOverride").value(true))
            .andExpect(jsonPath("$.decisionTimeMinutes").value(4))
            .andExpect(jsonPath("$.riskScore").value(100))
            .andExpect(jsonPath("$.riskLevel").value("CRITICAL"))
            .andExpect(jsonPath("$.reasons[*].type", hasItem("NO_EVIDENCE")))
            .andExpect(jsonPath("$.reasons[?(@.type == 'NO_EVIDENCE')].message", hasItem("Refund was approved without attached evidence, so the analyst cannot verify the customer's claim from this record.")))
            .andExpect(jsonPath("$.reasons[*].type", hasItem("MANUAL_OVERRIDE")))
            .andExpect(jsonPath("$.reasons[?(@.type == 'FAST_APPROVAL')].message", hasItem("Decision was approved in 4 minutes. This is at or below the 5-minute review threshold and may indicate a skipped check.")))
            .andExpect(jsonPath("$.relationFeatures.customerReturnCount", greaterThanOrEqualTo(5)))
            .andExpect(jsonPath("$.relationFeatures.agentApprovalRate").value(1.0))
            .andExpect(jsonPath("$.relationFeatures.customerAgentPairCount", greaterThanOrEqualTo(5)))
            .andExpect(jsonPath("$.relationFeatures.clusterSize", greaterThanOrEqualTo(5)))
            .andExpect(jsonPath("$.relationFeatures.refundAmountRatio").exists())
            .andExpect(jsonPath("$.relationFeatures.strongestRelationType").value("REPEATED_AGENT_CUSTOMER_PAIR"))
            .andExpect(jsonPath("$.relationFeatures.featureSource").value("CSV_DERIVED_FALLBACK"))
            .andExpect(jsonPath("$.calculatedAt").exists())
    }

    @Test
    fun `should return support agent risk summary through HTTP endpoint`() {
        mockMvc.perform(get("/api/scoring/agents/agent_777/risk-summary"))
            .andExpect(status().isOk)
            .andExpect(jsonPath("$.datasetId").value("demo"))
            .andExpect(jsonPath("$.agentId").value("agent_777"))
            .andExpect(jsonPath("$.totalApprovals", greaterThan(0)))
            .andExpect(jsonPath("$.totalReturns", greaterThan(0)))
            .andExpect(jsonPath("$.suspiciousApprovalsCount", greaterThan(0)))
            .andExpect(jsonPath("$.averageRiskScore", greaterThan(0.0)))
            .andExpect(jsonPath("$.topRiskReasons").isArray)
            .andExpect(jsonPath("$.topReason").exists())
            .andExpect(jsonPath("$.calculatedAt").exists())
    }

    @Test
    fun `should return dataset aware support agent risk summary through HTTP endpoint`() {
        mockMvc.perform(get("/api/scoring/datasets/demo/agents/agent_777/risk-summary"))
            .andExpect(status().isOk)
            .andExpect(jsonPath("$.agentId").value("agent_777"))
            .andExpect(jsonPath("$.datasetId").value("demo"))
            .andExpect(jsonPath("$.suspiciousApprovalsCount", greaterThan(0)))
            .andExpect(jsonPath("$.averageRiskScore", greaterThan(0.0)))
    }

    @Test
    fun `should return clean JSON error for unknown dataset`() {
        mockMvc.perform(get("/api/scoring/datasets/missing-dataset/suspicious-approvals"))
            .andExpect(status().isNotFound)
            .andExpect(jsonPath("$.status").value(404))
            .andExpect(jsonPath("$.error").value("Not Found"))
            .andExpect(jsonPath("$.message").value("Dataset was not found: missing-dataset"))
            .andExpect(jsonPath("$.path").value("/api/scoring/datasets/missing-dataset/suspicious-approvals"))
    }

    @Test
    fun `should return clean JSON error for unknown return`() {
        mockMvc.perform(get("/api/scoring/datasets/demo/returns/missing_return/details"))
            .andExpect(status().isNotFound)
            .andExpect(jsonPath("$.status").value(404))
            .andExpect(jsonPath("$.message").value("Return approval was not found: missing_return in dataset: demo"))
    }
}
