package com.antifrod.scoring.config

import org.springframework.amqp.core.Queue
import org.springframework.amqp.rabbit.connection.ConnectionFactory
import org.springframework.amqp.rabbit.core.RabbitAdmin
import org.springframework.amqp.support.converter.Jackson2JsonMessageConverter
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
    fun relationsBuiltQueue(): Queue {
        return Queue(REFUND_RELATIONS_BUILT_QUEUE, true)
    }

    @Bean
    fun rabbitAdmin(connectionFactory: ConnectionFactory): RabbitAdmin {
        return RabbitAdmin(connectionFactory).also { admin ->
            admin.setAutoStartup(true)
        }
    }

    @Bean
    fun jsonMessageConverter(): Jackson2JsonMessageConverter {
        return Jackson2JsonMessageConverter()
    }
}
