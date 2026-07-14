package com.antifrod.scoring.service

data class ScoringProcessingResult(
    val scoredApprovalsCount: Int,
    val suspiciousApprovalsCount: Int,
    val duplicate: Boolean = false
)
