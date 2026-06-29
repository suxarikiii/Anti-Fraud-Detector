package com.antifrod.scoring.repository

import com.antifrod.scoring.model.RefundApprovalRecord
import org.springframework.stereotype.Repository
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.Paths

@Repository
class RefundDatasetRepository {

    fun findAll(): List<RefundApprovalRecord> {
        val datasetPath = resolveDatasetPath()
        return Files.readAllLines(datasetPath)
            .drop(1)
            .filter { it.isNotBlank() }
            .map { line -> parseLine(line) }
    }

    fun findByReturnId(returnId: String): RefundApprovalRecord? {
        return findAll().firstOrNull { it.returnId == returnId }
    }

    fun findByDatasetId(datasetId: String): List<RefundApprovalRecord> {
        if (!isSupportedDatasetId(datasetId)) {
            return emptyList()
        }

        return findAll()
    }

    private fun isSupportedDatasetId(datasetId: String): Boolean {
        return datasetId == "demo" || UUID_REGEX.matches(datasetId)
    }

    private fun parseLine(line: String): RefundApprovalRecord {
        val columns = line.split(",")

        require(columns.size == 13) { "Expected 13 columns but got ${columns.size}" }

        return RefundApprovalRecord(
            orderId = columns[0].trim(),
            customerId = columns[1].trim(),
            returnId = columns[2].trim(),
            supportAgentId = columns[3].trim(),
            orderAmount = columns[4].trim().toDouble(),
            refundAmount = columns[5].trim().toDouble(),
            productCategory = columns[6].trim(),
            returnReason = columns[7].trim(),
            evidenceProvided = parseBoolean(columns[8]),
            decision = columns[9].trim().uppercase(),
            manualOverride = parseBoolean(columns[10]),
            decisionTimeMinutes = columns[11].trim().toInt(),
            timestamp = columns[12].trim()
        )
    }

    private fun parseBoolean(value: String): Boolean {
        return when (value.trim().lowercase()) {
            "true", "yes", "1" -> true
            "false", "no", "0" -> false
            else -> error("Invalid boolean value in refund dataset: $value")
        }
    }

    private fun resolveDatasetPath(): Path {
        val possiblePaths = listOf(
            Paths.get("/data/clean_refund_dataset.csv"),
            Paths.get("data/clean_refund_dataset.csv"),
            Paths.get("../../data/clean_refund_dataset.csv")
        )

        return possiblePaths.firstOrNull { Files.exists(it) }
            ?: error("clean_refund_dataset.csv was not found. " +
                    "Tried paths: ${possiblePaths.joinToString(", ") 
                    { it.toString() }}")
    }

    private companion object {
        val UUID_REGEX = Regex(
            "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"
        )
    }
}
