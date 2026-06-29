package com.antifrod.scoring.controller

import java.time.Instant

data class ApiErrorResponse(
    val status: Int,
    val error: String,
    val message: String,
    val path: String,
    val timestamp: Instant = Instant.now()
)
