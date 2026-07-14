package com.antifrod.scoring.service

import com.antifrod.scoring.repository.RefundDatasetRepository
import org.springframework.beans.factory.annotation.Value
import org.springframework.context.annotation.Primary
import org.springframework.stereotype.Component
import java.time.Instant

@Component
@Primary
class ProductionDatasetProvider(
    private val relationsDatasetProvider: RelationsDatasetProvider,
    @Value("\${app.scoring.demo-enabled:false}") private val demoEnabled: Boolean
) : DatasetProvider {
    private val demoRepository = RefundDatasetRepository()
    private val demoFeatures = CsvDerivedFeatureProvider()

    override fun load(datasetId: String): DatasetSnapshot {
        if (datasetId != "demo" || !demoEnabled) {
            return relationsDatasetProvider.load(datasetId)
        }
        val records = demoRepository.findByDatasetId(datasetId)
        if (records.isEmpty()) {
            throw ScoringNotFoundException("Dataset was not found: $datasetId")
        }
        return DatasetSnapshot(
            datasetId = datasetId,
            featureVersion = Instant.now().toEpochMilli(),
            featureSource = "DEMO_CSV",
            records = records,
            features = records.associate { it.returnId to demoFeatures.buildFeatures(it, records) }
        )
    }
}
