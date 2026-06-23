package com.antifrod.scoring.config

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import org.springframework.amqp.core.TopicExchange
import org.springframework.amqp.support.converter.Jackson2JsonMessageConverter

class RabbitMqConfigTest {

    private val config = RabbitMqConfig()

    @Test
    fun `should declare pipeline exchange as topic exchange`() {
        val exchange = config.pipelineExchange()

        assertIs<TopicExchange>(exchange)
        assertEquals(RabbitMqConfig.PIPELINE_EXCHANGE, exchange.name)
        assertEquals("topic", exchange.type)
    }

    @Test
    fun `should use json message converter for rabbit events`() {
        val converter = config.jsonMessageConverter()

        assertIs<Jackson2JsonMessageConverter>(converter)
    }
}
