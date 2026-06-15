package com.antifrod.scoring.controller

import com.antifrod.scoring.model.RecalculateResponse
import com.antifrod.scoring.service.ScoringService
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RestController

@RestController
@RequestMapping("/api/scoring")
class ScoringController(private val scoringService: ScoringService) {
    @GetMapping("/datasets/{datasetId}/suspicious-approvals")
    fun getSuspiciousApprovals(@PathVariable datasetId: String): List<SuspiciousRefundApproval> {
        return scoringService.getSuspiciousApprovals(datasetId)
    }

    @GetMapping("/returns/{returnId}/risk")
    fun getReturnRisk(@PathVariable returnId: String): RefundApprovalRiskScore {
        return scoringService.getReturnRisk(returnId)
    }

    @GetMapping("/agents/{agentId}/risk-summary")
    fun getAgentRiskSummary(@PathVariable agentId: String): AgentRiskSummary {
        return scoringService.getAgentRiskSummary(agentId)
    }

    @PostMapping("/datasets/{datasetId}/recalculate")
    fun recalculateDataset(@PathVariable datasetId: String): RecalculateResponse {
        return scoringService.recalculateDataset(datasetId)
    }
}