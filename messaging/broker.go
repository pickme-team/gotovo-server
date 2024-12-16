package messaging

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	RegisterQueueName = "user.create"
	CallbackQueueName = ""
)

var ErrChannelUnavailable = errors.New("messaging: broker channel unavailable")

const (
	// When setting up the channel after a channel exception
	reInitDelay = 2 * time.Second
)

type Broker struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	ready atomic.Bool

	connCloseCh chan *amqp.Error

	callbacks  map[string]chan []byte
	callbackMu sync.Mutex
}

func CreateBroker(url string) (*Broker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	connCloseCh := make(chan *amqp.Error)
	conn.NotifyClose(connCloseCh)

	b := &Broker{conn: conn, connCloseCh: connCloseCh}

	err = b.initChannel()
	if err != nil {
		return nil, err
	}

	b.ready.Store(true)
	return b, nil
}

func (b *Broker) Ready() bool {
	return b.ready.Load()
}

func (b *Broker) Close() error {
	if b.Ready() {
		err := b.ch.Close()
		if err != nil {
			return err
		}
	}

	err := b.conn.Close()
	if err != nil {
		return err
	}

	return nil
}

// SendMessage sends a message into the specified queue. That's kinda it.
func (b *Broker) SendMessage(queue string, body []byte) error {
	if !b.Ready() {
		return ErrChannelUnavailable
	}

	return b.ch.Publish("", queue, false, false, amqp.Publishing{
		ContentType: "application/octet-stream",
		Body:        body,
	})
}

// SendMessageRPCContext sends a message into the specified queue with a new correlation
// ID. It returns a channel where the caller can wait for a RPC response
// with a matching correlation ID.
func (b *Broker) SendMessageRPCContext(ctx context.Context, queue string, body []byte) (res []byte, err error) {
	if !b.Ready() {
		return nil, ErrChannelUnavailable
	}

	id := uuid.New()

	b.callbackMu.Lock()
	callback := make(chan []byte, 1)
	b.callbacks[id.String()] = callback
	b.callbackMu.Unlock()

	err = b.ch.Publish("", queue, false, false, amqp.Publishing{
		ContentType:   "application/octet-stream",
		CorrelationId: id.String(),
		ReplyTo:       CallbackQueueName,
		Body:          body,
	})
	if err != nil {
		return nil, err
	}

	select {
	case res := <-callback:
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendMessageRPC sends a message into the specified queue with a new correlation
// ID. It returns a channel where the caller can wait for a RPC response
// with a matching correlation ID.
func (b *Broker) SendMessageRPC(queue string, body []byte) (res []byte, err error) {
	return b.SendMessageRPCContext(context.Background(), queue, body)
}

// initChannel closes b.ch if it exists and initializes a new one, as well as a
// new callback listener. It should be called any time a channel op returns an
// error, as per the docs for amqp.Channel
func (b *Broker) initChannel() error {
	ch, err := b.conn.Channel()
	if err != nil {
		return err
	}

	err = declareQueues(ch)
	if err != nil {
		return err
	}

	closeChannel := make(chan *amqp.Error, 1)
	ch.NotifyClose(closeChannel)

	go func() {
		cause := <-closeChannel

		b.ready.Store(false)

		if cause != nil {
			slog.Error("messaging: channel closed due to error; reopening",
				"err", cause)
		}

		// Continuously reopen the channel until it works
		for err = b.initChannel(); err != nil; {
			slog.Error("messaging: failed to initialize channel; retrying")

			select {
			// Stop after connection close
			case <-b.connCloseCh:
				return
			case <-time.After(reInitDelay):
			}
		}

		b.ready.Store(true)
	}()

	return nil
}

func declareQueues(ch *amqp.Channel) error {
	_, err := ch.QueueDeclare(RegisterQueueName, true, false, false, false, nil)
	if err != nil {
		return err
	}

	return nil
}
