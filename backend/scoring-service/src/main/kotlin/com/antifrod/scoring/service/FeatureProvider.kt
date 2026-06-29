package com.antifrod.scoring.service

import com.antifrod.scoring.model.RefundApprovalFeatures
import com.antifrod.scoring.model.RefundApprovalRecord

interface FeatureProvider {
    fun buildFeatures(
        record: RefundApprovalRecord,
        allRecords: List<RefundApprovalRecord>
    ): RefundApprovalFeatures
}
