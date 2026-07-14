package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"upload-service/config"
)

type Consumer struct {
	conn     *amqp091.Connection
	channel  *amqp091.Channel
	cfg      config.RabbitConfig
	handler  func(context.Context, string, []byte) error
	logger   *slog.Logger
	confirms <-chan amqp091.Confirmation
}

func NewConsumer(cfg config.RabbitConfig, routingKeys []string, handler func(context.Context, string, []byte) error, logger *slog.Logger) (*Consumer, error) {
	conn, err := amqp091.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq consumer: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("create consumer channel: %w", err)
	}
	closeWith := func(err error) (*Consumer, error) { _ = ch.Close(); _ = conn.Close(); return nil, err }
	if err = ch.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		return closeWith(fmt.Errorf("declare exchange: %w", err))
	}
	if _, err = ch.QueueDeclare(cfg.DeadQueue, true, false, false, false, nil); err != nil {
		return closeWith(fmt.Errorf("declare dead-letter queue: %w", err))
	}
	args := amqp091.Table{"x-dead-letter-exchange": "", "x-dead-letter-routing-key": cfg.DeadQueue}
	if _, err = ch.QueueDeclare(cfg.EventsQueue, true, false, false, false, args); err != nil {
		return closeWith(fmt.Errorf("declare events queue: %w", err))
	}
	for _, key := range routingKeys {
		if err = ch.QueueBind(cfg.EventsQueue, key, cfg.Exchange, false, nil); err != nil {
			return closeWith(fmt.Errorf("bind %s: %w", key, err))
		}
	}
	if err = ch.Qos(1, 0, false); err != nil {
		return closeWith(fmt.Errorf("set consumer qos: %w", err))
	}
	if err = ch.Confirm(false); err != nil {
		return closeWith(fmt.Errorf("enable retry confirms: %w", err))
	}
	confirms := ch.NotifyPublish(make(chan amqp091.Confirmation, 1))
	return &Consumer{conn: conn, channel: ch, cfg: cfg, handler: handler, logger: logger, confirms: confirms}, nil
}

func (c *Consumer) Consume(ctx context.Context) error {
	deliveries, err := c.channel.Consume(c.cfg.EventsQueue, c.cfg.ConsumerName, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume pipeline events: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("pipeline event delivery channel closed")
			}
			c.process(ctx, delivery)
		}
	}
}

func (c *Consumer) process(ctx context.Context, delivery amqp091.Delivery) {
	err := c.handler(ctx, delivery.RoutingKey, delivery.Body)
	if err == nil {
		_ = delivery.Ack(false)
		return
	}
	permanent, isPermanent := err.(interface{ Permanent() bool })
	if isPermanent && permanent.Permanent() {
		if c.logger != nil {
			c.logger.Warn("pipeline event sent to DLQ", "routingKey", delivery.RoutingKey, "reason", err.Error())
		}
		_ = delivery.Nack(false, false)
		return
	}
	retries := headerInt(delivery.Headers["x-upload-retry-count"])
	if retries >= c.cfg.MaxRetries {
		if c.logger != nil {
			c.logger.Error("pipeline event retries exhausted", "routingKey", delivery.RoutingKey, "retries", retries, "error", err)
		}
		_ = delivery.Nack(false, false)
		return
	}
	headers := delivery.Headers
	if headers == nil {
		headers = amqp091.Table{}
	}
	headers["x-upload-retry-count"] = int32(retries + 1)
	publishErr := c.channel.PublishWithContext(ctx, c.cfg.Exchange, delivery.RoutingKey, false, false, amqp091.Publishing{Headers: headers, ContentType: "application/json", DeliveryMode: amqp091.Persistent, MessageId: delivery.MessageId, Timestamp: time.Now().UTC(), Body: delivery.Body})
	if publishErr != nil {
		_ = delivery.Nack(false, true)
		return
	}
	select {
	case confirmation, ok := <-c.confirms:
		if !ok || !confirmation.Ack {
			_ = delivery.Nack(false, true)
			return
		}
	case <-ctx.Done():
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}

func headerInt(value interface{}) int {
	switch v := value.(type) {
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	}
	return 0
}
func (c *Consumer) Close() {
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
