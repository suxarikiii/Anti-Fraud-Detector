package com.antifrod.scoring.messaging

import com.antifrod.scoring.messaging.event.PipelineFailedEvent
import com.antifrod.scoring.messaging.event.ScoringCompletedEvent
import org.springframework.amqp.rabbit.core.RabbitTemplate
import org.springframework.beans.factory.annotation.Value
import org.springframework.stereotype.Component

@Component
class ScoringEventPublisher(
    private val rabbitTemplate: RabbitTemplate,

    @Value("\${app.rabbitmq.exchange:pipeline.exchange}")
    private val exchange: String
) {

    fun publishScoringCompleted(event: ScoringCompletedEvent) {
        rabbitTemplate.convertAndSend(
            exchange,
            "refund.scoring.completed",
            event
        )
    }

    fun publishPipelineFailed(event: PipelineFailedEvent) {
        rabbitTemplate.convertAndSend(
            exchange,
            "pipeline.failed",
            event
        )
    }
}