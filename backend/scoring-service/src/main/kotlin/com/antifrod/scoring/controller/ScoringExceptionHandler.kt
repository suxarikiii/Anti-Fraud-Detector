package com.antifrod.scoring.controller

import com.antifrod.scoring.service.ScoringNotFoundException
import com.antifrod.scoring.service.ScoringDependencyException
import com.antifrod.scoring.service.ScoringValidationException
import jakarta.servlet.http.HttpServletRequest
import org.springframework.http.HttpStatus
import org.springframework.http.ResponseEntity
import org.springframework.web.bind.annotation.ExceptionHandler
import org.springframework.web.bind.annotation.RestControllerAdvice
import org.springframework.http.converter.HttpMessageNotReadableException
import org.springframework.web.method.annotation.MethodArgumentTypeMismatchException
import org.springframework.web.bind.MethodArgumentNotValidException

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

    @ExceptionHandler(ScoringDependencyException::class)
    fun handleDependency(
        exception: ScoringDependencyException,
        request: HttpServletRequest
    ): ResponseEntity<ApiErrorResponse> = ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).body(
        ApiErrorResponse(
            status = HttpStatus.SERVICE_UNAVAILABLE.value(),
            error = HttpStatus.SERVICE_UNAVAILABLE.reasonPhrase,
            message = exception.message ?: "Scoring dependency is unavailable",
            path = request.requestURI,
            errorCode = exception.errorCode
        )
    )

    @ExceptionHandler(ScoringValidationException::class)
    fun handleValidation(
        exception: ScoringValidationException,
        request: HttpServletRequest
    ): ResponseEntity<ApiErrorResponse> = ResponseEntity.badRequest().body(
        ApiErrorResponse(
            status = HttpStatus.BAD_REQUEST.value(),
            error = HttpStatus.BAD_REQUEST.reasonPhrase,
            message = exception.message ?: "Invalid scoring request",
            path = request.requestURI,
            errorCode = "VALIDATION_ERROR"
        )
    )

    @ExceptionHandler(
        HttpMessageNotReadableException::class,
        MethodArgumentTypeMismatchException::class,
        MethodArgumentNotValidException::class
    )
    fun handleInvalidRequest(
        exception: Exception,
        request: HttpServletRequest
    ): ResponseEntity<ApiErrorResponse> = ResponseEntity.badRequest().body(
        ApiErrorResponse(
            status = HttpStatus.BAD_REQUEST.value(),
            error = HttpStatus.BAD_REQUEST.reasonPhrase,
            message = "Request contains an invalid enum or field value",
            path = request.requestURI,
            errorCode = "INVALID_REQUEST"
        )
    )
}
