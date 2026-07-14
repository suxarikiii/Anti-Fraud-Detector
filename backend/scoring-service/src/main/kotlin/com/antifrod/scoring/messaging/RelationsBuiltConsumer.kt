package com.antifrod.scoring.messaging

import com.antifrod.scoring.config.RabbitMqConfig
import com.antifrod.scoring.messaging.event.PipelineFailedEvent
import com.antifrod.scoring.messaging.event.RelationsBuiltEvent
import com.antifrod.scoring.messaging.event.ScoringCompletedEvent
import com.antifrod.scoring.service.ScoringService
import org.springframework.amqp.rabbit.annotation.RabbitListener
import org.springframework.stereotype.Component
import java.time.Instant

@Component
class RelationsBuiltConsumer(
    private val scoringService: ScoringService,
    private val scoringEventPublisher: ScoringEventPublisher
) {

    @RabbitListener(queues = [RabbitMqConfig.REFUND_RELATIONS_BUILT_QUEUE])
    fun handleRelationsBuilt(event: RelationsBuiltEvent) {
        try {
            val result = scoringService.processRelationsBuilt(event)

            scoringEventPublisher.publishScoringCompleted(
                ScoringCompletedEvent(
                    datasetId = event.datasetId,
                    jobId = event.jobId,
                    scoredApprovalsCount = result.scoredApprovalsCount,
                    suspiciousApprovalsCount = result.suspiciousApprovalsCount,
                    timestamp = Instant.now()
                )
            )
        } catch (exception: Exception) {
            scoringEventPublisher.publishPipelineFailed(
                PipelineFailedEvent(
                    datasetId = event.datasetId,
                    jobId = event.jobId,
                    failedStep = "SCORING",
                    errorCode = (exception as? com.antifrod.scoring.service.ScoringDependencyException)?.errorCode
                        ?: "SCORING_FAILED",
                    errorMessage = exception.message ?: "Unknown scoring error",
                    timestamp = Instant.now()
                )
            )
        }
    }
}
