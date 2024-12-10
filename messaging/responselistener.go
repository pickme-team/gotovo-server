package messaging

import (
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type responseListener struct {
	// channels contains channels to send RPC responses to.
	channels map[string]chan<- []byte
	mu       sync.Mutex
}

// newResponseListener assumes the callback queue is already declared.
func newResponseListener(ch *amqp.Channel) (*responseListener, error) {
	msgs, err := ch.Consume(CallbackQueueName, "", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	listener := &responseListener{channels: make(map[string]chan<- []byte)}
	go listener.consumeResponses(msgs)

	return listener, nil
}

func (l *responseListener) addCallback(id string) <-chan []byte {
	ch := make(chan []byte)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.channels[id] = ch
	return ch
}

func (l *responseListener) consumeResponses(msgs <-chan amqp.Delivery) {
	for msg := range msgs {
		l.mu.Lock()
		ch, ok := l.channels[msg.CorrelationId]
		l.mu.Unlock()

		if !ok {
			slog.Error("messaging: RPC response arrived with no callback found",
				"msg", msg)
			continue
		}

		ch <- msg.Body
	}
}
