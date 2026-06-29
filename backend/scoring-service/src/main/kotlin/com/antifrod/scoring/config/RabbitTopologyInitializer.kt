package com.antifrod.scoring.config

import com.rabbitmq.client.Channel
import org.slf4j.LoggerFactory
import org.springframework.amqp.rabbit.connection.ConnectionFactory
import org.springframework.boot.context.event.ApplicationReadyEvent
import org.springframework.context.event.EventListener
import org.springframework.stereotype.Component
import java.io.IOException

@Component
class RabbitTopologyInitializer(
    private val connectionFactory: ConnectionFactory
) {

    private val logger = LoggerFactory.getLogger(RabbitTopologyInitializer::class.java)

    @EventListener(ApplicationReadyEvent::class)
    fun initializeTopology() {
        val connection = connectionFactory.createConnection()
        var channel: Channel? = null

        try {
            channel = connection.createChannel(false)
            channel.queueDeclare(
                RabbitMqConfig.REFUND_RELATIONS_BUILT_QUEUE,
                true,
                false,
                false,
                null
            )
            channel = ensureExchange(channel, connection)
            channel.queueBind(
                RabbitMqConfig.REFUND_RELATIONS_BUILT_QUEUE,
                RabbitMqConfig.PIPELINE_EXCHANGE,
                RabbitMqConfig.REFUND_RELATIONS_BUILT_ROUTING_KEY
            )
        } catch (exception: Exception) {
            logger.warn("RabbitMQ scoring topology initialization did not complete: {}", exception.message)
        } finally {
            try {
                channel?.close()
            } catch (_: Exception) {
            }
            connection.close()
        }
    }

    private fun ensureExchange(
        channel: Channel,
        connection: org.springframework.amqp.rabbit.connection.Connection
    ): Channel {
        return try {
            channel.exchangeDeclarePassive(RabbitMqConfig.PIPELINE_EXCHANGE)
            channel
        } catch (exception: IOException) {
            if (!isMissingExchange(exception)) {
                throw exception
            }

            try {
                channel.close()
            } catch (_: Exception) {
            }

            connection.createChannel(false).also { retryChannel ->
                retryChannel.exchangeDeclare(
                    RabbitMqConfig.PIPELINE_EXCHANGE,
                    "topic",
                    true,
                    false,
                    false,
                    null
                )
            }
        }
    }

    private fun isMissingExchange(exception: IOException): Boolean {
        return exception.message?.contains("NOT_FOUND") == true ||
                exception.cause?.message?.contains("NOT_FOUND") == true
    }
}
