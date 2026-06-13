package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Connection wraps an AMQP connection and a single channel.
type Connection struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// Dial opens a connection and a channel to the broker at url.
func Dial(url string) (*Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	return &Connection{conn: conn, ch: ch}, nil
}

// Channel returns the underlying AMQP channel.
func (c *Connection) Channel() *amqp.Channel {
	return c.ch
}

// Close closes the channel and the connection.
func (c *Connection) Close() error {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
