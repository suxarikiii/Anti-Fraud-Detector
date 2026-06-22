package com.antifrod.scoring.config

import org.springframework.amqp.core.DirectExchange
import org.springframework.amqp.core.Queue
import org.springframework.amqp.core.Binding
import org.springframework.amqp.core.BindingBuilder
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration


@Configuration
class RabbitMqConfig {
    companion object {
        const val PIPELINE_EXCHANGE = "pipeline.exchange"
        const val REFUND_RELATIONS_BUILT_QUEUE = "scoring.refund-relations-built.queue"
        const val REFUND_RELATIONS_BUILT_ROUTING_KEY = "refund.relations.built"
    }

    @Bean
    fun pipelineExchange(): DirectExchange {
        return DirectExchange(PIPELINE_EXCHANGE)
    }

    @Bean
    fun relationsBuiltQueue(): Queue {
        return Queue(REFUND_RELATIONS_BUILT_QUEUE, true)
    }

    @Bean
    fun relationsBuiltBinding(
        relationsBuiltQueue: Queue,
        pipelineExchange: DirectExchange
    ): Binding {
        return BindingBuilder
            .bind(relationsBuiltQueue)
            .to(pipelineExchange)
            .with(REFUND_RELATIONS_BUILT_ROUTING_KEY)
    }
}
