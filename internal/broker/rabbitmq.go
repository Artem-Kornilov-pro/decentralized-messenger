package broker

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// exchangeName is the topic exchange chat-log events are published to.
const exchangeName = "messenger.log.events"

// RabbitMQ is a Broker backed by RabbitMQ. Events are published to a durable
// topic exchange, routed by "<kind>.<chatID>", for guaranteed delivery of chat
// state updates between nodes.
type RabbitMQ struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// compile-time assertion that RabbitMQ satisfies the Broker port.
var _ Broker = (*RabbitMQ)(nil)

// NewRabbitMQ dials the given AMQP URL and declares the durable topic exchange.
func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	return &RabbitMQ{conn: conn, ch: ch}, nil
}

// Close tears down the channel and connection.
func (r *RabbitMQ) Close() error {
	if r.ch != nil {
		_ = r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// Publish marshals the event to JSON and publishes it as a persistent message.
func (r *RabbitMQ) Publish(event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	routingKey := string(event.Kind) + "." + event.ChatID
	return r.ch.PublishWithContext(ctx, exchangeName, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
	})
}

// Subscribe declares an anonymous, exclusive queue bound to every routing key
// on the exchange and streams decoded events to the returned channel. The
// queue and its consumer are torn down when cancel is called.
func (r *RabbitMQ) Subscribe() (<-chan Event, func(), error) {
	q, err := r.ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return nil, nil, err
	}
	if err := r.ch.QueueBind(q.Name, "#", exchangeName, false, nil); err != nil {
		return nil, nil, err
	}
	consumerTag := "messenger-sub-" + q.Name
	deliveries, err := r.ch.Consume(q.Name, consumerTag, true, true, false, false, nil)
	if err != nil {
		return nil, nil, err
	}

	out := make(chan Event, 64)
	done := make(chan struct{})
	go func() {
		defer close(out)
		for {
			select {
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				var evt Event
				if err := json.Unmarshal(d.Body, &evt); err != nil {
					continue
				}
				select {
				case out <- evt:
				default:
				}
			case <-done:
				return
			}
		}
	}()

	cancel := func() {
		close(done)
		_ = r.ch.Cancel(consumerTag, false)
	}
	return out, cancel, nil
}
