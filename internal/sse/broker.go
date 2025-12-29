package sse

import "sync"

type Broker[T any] struct {
	mu      sync.RWMutex
	clients map[chan T]struct{}
}

func NewBroker[T any]() *Broker[T] {
	return &Broker[T]{
		clients: make(map[chan T]struct{}),
	}
}

func (b *Broker[T]) Subscribe(buffer int) chan T {
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan T, buffer)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker[T]) Unsubscribe(ch chan T) {
	b.mu.Lock()
	if _, ok := b.clients[ch]; ok {
		delete(b.clients, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *Broker[T]) Publish(msg T) {
	b.mu.RLock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
	b.mu.RUnlock()
}

// SubscriberCount returns the number of active subscribers.
func (b *Broker[T]) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
