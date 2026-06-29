package com.antifrod.scoring.config

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertIs
import org.springframework.amqp.core.Queue
import org.springframework.amqp.support.converter.Jackson2JsonMessageConverter

class RabbitMqConfigTest {

    private val config = RabbitMqConfig()

    @Test
    fun `should use json message converter for rabbit events`() {
        val converter = config.jsonMessageConverter()

        assertIs<Jackson2JsonMessageConverter>(converter)
    }

    @Test
    fun `should declare durable relations built queue`() {
        val queue = config.relationsBuiltQueue()

        assertIs<Queue>(queue)
        assertEquals(RabbitMqConfig.REFUND_RELATIONS_BUILT_QUEUE, queue.name)
        assertEquals(true, queue.isDurable)
    }
}
