package com.antifrod.scoring.controller

import com.antifrod.scoring.service.ScoringNotFoundException
import jakarta.servlet.http.HttpServletRequest
import org.springframework.http.HttpStatus
import org.springframework.http.ResponseEntity
import org.springframework.web.bind.annotation.ExceptionHandler
import org.springframework.web.bind.annotation.RestControllerAdvice

@RestControllerAdvice
class ScoringExceptionHandler {

    @ExceptionHandler(ScoringNotFoundException::class)
    fun handleNotFound(
        exception: ScoringNotFoundException,
        request: HttpServletRequest
    ): ResponseEntity<ApiErrorResponse> {
        return ResponseEntity.status(HttpStatus.NOT_FOUND).body(
            ApiErrorResponse(
                status = HttpStatus.NOT_FOUND.value(),
                error = HttpStatus.NOT_FOUND.reasonPhrase,
                message = exception.message ?: "Scoring resource was not found",
                path = request.requestURI
            )
        )
    }
}
