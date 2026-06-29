package com.antifrod.scoring.config

import org.springframework.amqp.core.Queue
import org.springframework.amqp.core.Binding
import org.springframework.amqp.core.BindingBuilder
import org.springframework.amqp.core.TopicExchange
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration


@Configuration
class RabbitMqConfig {
    @Bean
    fun pipelineExchange(): TopicExchange {
        return TopicExchange("pipeline.exchange")
    }

    @Bean
    fun relationsBuiltQueue(): Queue {
        return Queue("scoring.relations-built.queue", true)
    }

    @Bean
    fun relationsBuiltBinding(
        relationsBuiltQueue: Queue,
        pipelineExchange: TopicExchange
    ): Binding {
        return BindingBuilder
            .bind(relationsBuiltQueue)
            .to(pipelineExchange)
            .with("refund.relations.built")
    }
}
