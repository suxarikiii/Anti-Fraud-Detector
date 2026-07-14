package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn     *amqp091.Connection
	channel  *amqp091.Channel
	exchange string
	confirms <-chan amqp091.Confirmation
	mu       sync.Mutex
}

func NewPublisher(url, exchange string) (*Publisher, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("create channel: %w", err)
	}

	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare exchange: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}

	return &Publisher{conn: conn, channel: ch, exchange: exchange, confirms: ch.NotifyPublish(make(chan amqp091.Confirmation, 1))}, nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.channel.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		MessageId:    uuid.NewString(),
		Timestamp:    time.Now().UTC(),
		Body:         body,
	}); err != nil {
		return err
	}
	select {
	case confirmation, ok := <-p.confirms:
		if !ok || !confirmation.Ack {
			return fmt.Errorf("rabbitmq did not confirm published event")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Publisher) Close() {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}
