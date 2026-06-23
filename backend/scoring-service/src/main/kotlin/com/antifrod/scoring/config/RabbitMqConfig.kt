package com.antifrod.scoring.config

import org.springframework.amqp.core.Binding
import org.springframework.amqp.core.BindingBuilder
import org.springframework.amqp.core.Queue
import org.springframework.amqp.core.TopicExchange
import org.springframework.amqp.support.converter.Jackson2JsonMessageConverter
import org.springframework.amqp.support.converter.MessageConverter
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration

@Configuration
class RabbitMqConfig {
    companion object {
        const val PIPELINE_EXCHANGE = "pipeline.exchange"
        const val REFUND_RELATIONS_BUILT_QUEUE = "scoring.refund-relations-built.queue"
        const val REFUND_RELATIONS_BUILT_ROUTING_KEY = "refund.relations.built"
        const val REFUND_SCORING_COMPLETED_ROUTING_KEY = "refund.scoring.completed"
        const val PIPELINE_FAILED_ROUTING_KEY = "pipeline.failed"
    }

    @Bean
    fun pipelineExchange(): TopicExchange {
        return TopicExchange(PIPELINE_EXCHANGE)
    }

    @Bean
    fun relationsBuiltQueue(): Queue {
        return Queue(REFUND_RELATIONS_BUILT_QUEUE, true)
    }

    @Bean
    fun relationsBuiltBinding(
        relationsBuiltQueue: Queue,
        pipelineExchange: TopicExchange
    ): Binding {
        return BindingBuilder
            .bind(relationsBuiltQueue)
            .to(pipelineExchange)
            .with(REFUND_RELATIONS_BUILT_ROUTING_KEY)
    }

    @Bean
    fun jsonMessageConverter(): MessageConverter {
        return Jackson2JsonMessageConverter()
    }
}
