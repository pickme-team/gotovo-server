package messaging

import (
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	RegisterQueueName = "user.create"
	CallbackQueueName = ""
)

type Broker struct {
	conn *amqp.Connection
	ch   *amqp.Channel

	listener *responseListener
}

func CreateBroker(url string) (*Broker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	b := &Broker{conn: conn}

	err = b.initChannel()
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (b *Broker) Close() error {
	err := b.conn.Close()
	if err != nil {
		return err
	}

	if b.ch != nil {
		return b.ch.Close()
	}

	return nil
}

// SendMessage sends a message into the specified queue. That's kinda it.
func (b *Broker) SendMessage(queue string, body []byte) error {
	return b.ch.Publish("", queue, false, false, amqp.Publishing{
		ContentType: "application/octet-stream",
		Body:        body,
	})
}

// SendMessageRPC sends a message into the specified queue with a new correlation
// ID. It returns a channel where the caller can wait for a RPC response
// with a matching correlation ID.
func (b *Broker) SendMessageRPC(queue string, body []byte) (res <-chan []byte, err error) {
	id := uuid.New()

	res = b.listener.addCallback(id.String())

	err = b.ch.Publish("", queue, false, false, amqp.Publishing{
		ContentType:   "application/octet-stream",
		CorrelationId: id.String(),
		ReplyTo:       CallbackQueueName,
		Body:          body,
	})

	return res, err
}

// initChannel closes b.ch if it exists and initializes a new one, as well as a
// new callback listener. It should be called any time a channel op returns an
// error, as per the docs for amqp.Channel
func (b *Broker) initChannel() error {
	// Close the previous channel
	if b.ch != nil {
		b.ch.Close()
	}

	ch, err := b.conn.Channel()
	if err != nil {
		return err
	}

	err = declareQueues(ch)
	if err != nil {
		return err
	}

	// Create a new listener for the new channel
	listener, err := newResponseListener(ch)
	if err != nil {
		return err
	}

	// If there are any callback channels in the existing listener,
	// move them into the new one
	if b.listener != nil {
		b.listener.mu.Lock()
		if len(b.listener.channels) > 0 {
			listener.channels = b.listener.channels
		}
		b.listener.mu.Unlock()
	}

	b.ch = ch
	b.listener = listener
	return nil
}

func declareQueues(ch *amqp.Channel) error {
	_, err := ch.QueueDeclare(RegisterQueueName, true, false, false, false, nil)
	if err != nil {
		return err
	}

	return nil
}
