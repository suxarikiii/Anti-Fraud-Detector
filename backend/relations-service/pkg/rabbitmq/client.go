package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rabbitmq/amqp091-go"

	"relations-service/config"
	"relations-service/internal/service"
)

type Client struct {
	conn *amqp091.Connection
	ch   *amqp091.Channel
	cfg  config.RabbitConfig
}

func NewClient(cfg config.RabbitConfig) (*Client, error) {
	conn, err := amqp091.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("create channel: %w", err)
	}

	if err := ch.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	if _, err := ch.QueueDeclare(cfg.NormalizedQueue, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare normalized queue: %w", err)
	}

	if err := ch.QueueBind(cfg.NormalizedQueue, cfg.NormalizedRoutingKey, cfg.Exchange, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("bind normalized queue: %w", err)
	}

	return &Client{conn: conn, ch: ch, cfg: cfg}, nil
}

func (c *Client) ConsumeNormalizedDatasets(ctx context.Context, handler func(context.Context, service.NormalizedDatasetEvent) error) error {
	deliveries, err := c.ch.Consume(c.cfg.NormalizedQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume normalized queue: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return nil
			}

			var event service.NormalizedDatasetEvent
			if err := json.Unmarshal(delivery.Body, &event); err != nil {
				_ = delivery.Nack(false, false)
				continue
			}

			if err := handler(ctx, event); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}

			_ = delivery.Ack(false)
		}
	}
}

func (c *Client) PublishRelationsBuilt(ctx context.Context, event service.RelationsBuiltEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal relations built event: %w", err)
	}

	return c.ch.PublishWithContext(ctx, c.cfg.Exchange, c.cfg.RelationsBuiltRoutingKey, false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func (c *Client) PublishPipelineFailed(ctx context.Context, event service.PipelineFailedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal pipeline failed event: %w", err)
	}

	return c.ch.PublishWithContext(ctx, c.cfg.Exchange, "pipeline.failed", false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func (c *Client) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
