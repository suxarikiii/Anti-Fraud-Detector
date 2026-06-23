package com.antifrod.scoring.messaging

import com.antifrod.scoring.config.RabbitMqConfig
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
            RabbitMqConfig.REFUND_SCORING_COMPLETED_ROUTING_KEY,
            event
        )
    }

    fun publishPipelineFailed(event: PipelineFailedEvent) {
        rabbitTemplate.convertAndSend(
            exchange,
            RabbitMqConfig.PIPELINE_FAILED_ROUTING_KEY,
            event
        )
    }
}
