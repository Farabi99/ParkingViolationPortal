package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"parking-portal/pkg/logger"
)

type Client struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func Connect(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	return &Client{
		conn: conn,
		ch:   ch,
	}, nil
}

func (c *Client) Close() {
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) SetupQueue(queueName, dlqName string) error {
	// Setup DLX
	err := c.ch.ExchangeDeclare(
		"dlx", "direct", true, false, false, false, nil,
	)
	if err != nil {
		return err
	}

	_, err = c.ch.QueueDeclare(
		dlqName, true, false, false, false, nil,
	)
	if err != nil {
		return err
	}

	err = c.ch.QueueBind(dlqName, "", "dlx", false, nil)
	if err != nil {
		return err
	}

	// Setup main queue with DLX args
	args := amqp.Table{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "",
	}
	_, err = c.ch.QueueDeclare(
		queueName, true, false, false, false, args,
	)
	return err
}

func (c *Client) Publish(ctx context.Context, queueName string, body interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	correlationID := ""
	if reqID, ok := ctx.Value(logger.CorrelationIDKey).(string); ok {
		correlationID = reqID
	}

	err = c.ch.PublishWithContext(ctx,
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			Body:          b,
			CorrelationId: correlationID,
		})

	if err != nil {
		logger.Error(ctx, "Failed to publish message", "error", err, "queue", queueName)
		return err
	}
	return nil
}

func (c *Client) Consume(queueName string) (<-chan amqp.Delivery, error) {
	return c.ch.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
}
