package com.antifrod.scoring.controller

import com.antifrod.scoring.model.AgentRiskSummary
import com.antifrod.scoring.model.InvestigationDecision
import com.antifrod.scoring.model.InvestigationDecisionRequest
import com.antifrod.scoring.model.InvestigationOutcome
import com.antifrod.scoring.model.RecalculateResponse
import com.antifrod.scoring.model.RefundApprovalDetailsResponse
import com.antifrod.scoring.model.RefundApprovalRiskScore
import com.antifrod.scoring.model.RiskLevel
import com.antifrod.scoring.model.SuspiciousRefundApproval
import com.antifrod.scoring.service.ScoringService
import jakarta.validation.Valid
import org.springframework.http.HttpHeaders
import org.springframework.http.MediaType
import org.springframework.http.ResponseEntity
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.PutMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RequestParam
import org.springframework.web.bind.annotation.RestController

@RestController
@RequestMapping("/api/scoring")
class ScoringController(
    private val scoringService: ScoringService
) {

    @GetMapping("/datasets/{datasetId}/suspicious-approvals")
    fun getSuspiciousApprovals(
        @PathVariable datasetId: String,
        @RequestParam(required = false) risk: RiskLevel?,
        @RequestParam(required = false) agent: String?,
        @RequestParam(required = false) outcome: InvestigationOutcome?
    ): List<SuspiciousRefundApproval> {
        return scoringService.getSuspiciousApprovals(datasetId, risk, agent, outcome)
    }

    @GetMapping("/returns/{returnId}/risk")
    fun getReturnRisk(
        @PathVariable returnId: String
    ): RefundApprovalRiskScore {
        return scoringService.getReturnRisk(returnId)
    }

    @GetMapping("/datasets/{datasetId}/returns/{returnId}/risk")
    fun getReturnRiskByDataset(
        @PathVariable datasetId: String,
        @PathVariable returnId: String
    ): RefundApprovalRiskScore {
        return scoringService.getReturnRisk(datasetId, returnId)
    }

    @GetMapping("/agents/{agentId}/risk-summary")
    fun getAgentRiskSummary(
        @PathVariable agentId: String
    ): AgentRiskSummary {
        return scoringService.getAgentRiskSummary(agentId)
    }

    @GetMapping("/datasets/{datasetId}/agents/{agentId}/risk-summary")
    fun getAgentRiskSummaryByDataset(
        @PathVariable datasetId: String,
        @PathVariable agentId: String
    ): AgentRiskSummary {
        return scoringService.getAgentRiskSummary(datasetId, agentId)
    }

    @GetMapping("/datasets/{datasetId}/returns/{returnId}/details")
    fun getReturnDetails(
        @PathVariable datasetId: String,
        @PathVariable returnId: String
    ): RefundApprovalDetailsResponse {
        return scoringService.getReturnDetails(datasetId, returnId)
    }

    @PostMapping("/datasets/{datasetId}/recalculate")
    fun recalculateDataset(
        @PathVariable datasetId: String
    ): RecalculateResponse {
        return scoringService.recalculateDataset(datasetId)
    }

    @GetMapping("/datasets/{datasetId}/returns/{returnId}/decision")
    fun getDecision(
        @PathVariable datasetId: String,
        @PathVariable returnId: String
    ): InvestigationDecision = scoringService.getDecision(datasetId, returnId)

    @PutMapping("/datasets/{datasetId}/returns/{returnId}/decision")
    fun saveDecision(
        @PathVariable datasetId: String,
        @PathVariable returnId: String,
        @Valid @RequestBody request: InvestigationDecisionRequest
    ): InvestigationDecision = scoringService.saveDecision(datasetId, returnId, request)

    @GetMapping("/datasets/{datasetId}/export.csv", produces = ["text/csv;charset=UTF-8"])
    fun exportDataset(
        @PathVariable datasetId: String,
        @RequestParam(required = false) risk: RiskLevel?,
        @RequestParam(required = false) agent: String?,
        @RequestParam(required = false) outcome: InvestigationOutcome?
    ): ResponseEntity<String> = ResponseEntity.ok()
        .contentType(MediaType("text", "csv", Charsets.UTF_8))
        .header(HttpHeaders.CONTENT_DISPOSITION, "attachment; filename=\"scoring-$datasetId.csv\"")
        .body(scoringService.exportCsv(datasetId, risk, agent, outcome))
}
