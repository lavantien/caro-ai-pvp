package server

import (
	"errors"
	"sync"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Event is the opaque realtime payload the Hub fans out. Kind selects the
// stream (bot log line, board update, ...) and Payload carries the line, so
// later consumers ride this shape without rework.
type Event struct {
	Kind    string
	Payload string
}

var (
	// ErrHubClosed rejects Subscribe after Hub.Close and explains every
	// stream the close ended.
	ErrHubClosed = errors.New("server: hub closed")
	// ErrSlowConsumer explains a stream Publish closed because its buffer
	// was full: the consumer fell config.HubSubscriberBuffer events behind.
	ErrSlowConsumer = errors.New("server: hub subscriber too slow, evicted")
)

// Hub is the realtime stats pipeline: per-room subscriber sets fanned out in
// publish order. Publish never waits on a consumer: a subscriber whose
// buffer is full is evicted and its stream closed with ErrSlowConsumer. The
// game log stays the source of truth, a stalled watcher reconnects.
type Hub struct {
	mu     sync.Mutex
	rooms  map[string]map[*subscriber]struct{}
	closed bool
}

type subscriber struct {
	room   string
	ch     chan Event
	closed bool
	err    error
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[*subscriber]struct{})}
}

// Subscribe registers a watcher of room and returns its event stream,
// buffered with config.HubSubscriberBuffer slots. Err on the subscription
// explains why the stream ended: ErrSlowConsumer after eviction,
// ErrHubClosed after Hub.Close, nil after a voluntary Unsubscribe.
func (h *Hub) Subscribe(room string) (*Subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrHubClosed
	}
	s := &subscriber{
		room: room,
		ch:   make(chan Event, config.HubSubscriberBuffer),
	}
	set := h.rooms[room]
	if set == nil {
		set = make(map[*subscriber]struct{}, 1)
		h.rooms[room] = set
	}
	set[s] = struct{}{}
	return &Subscription{hub: h, s: s}, nil
}

// Publish delivers ev to every current subscriber of room in one pass held
// under the hub lock, so all subscribers of a room observe the same publish
// order and cross-room ordering stays unconstrained. The hot path performs
// only a map lookup and non-blocking channel sends.
func (h *Hub) Publish(room string, ev Event) {
	h.mu.Lock()
	set := h.rooms[room]
	if len(set) == 0 {
		h.mu.Unlock()
		return
	}
	for s := range set {
		select {
		case s.ch <- ev:
		default:
			delete(set, s)
			s.closed = true
			s.err = ErrSlowConsumer
			close(s.ch)
		}
	}
	if len(set) == 0 {
		delete(h.rooms, room)
	}
	h.mu.Unlock()
}

// Close closes every subscriber stream with ErrHubClosed and rejects further
// Subscribe with the same error. It is idempotent; Publish afterwards is a
// no-op.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, set := range h.rooms {
		for s := range set {
			s.closed = true
			s.err = ErrHubClosed
			close(s.ch)
		}
	}
	h.rooms = make(map[string]map[*subscriber]struct{})
}

// Subscription is one Hub watcher's handle.
type Subscription struct {
	hub *Hub
	s   *subscriber
}

// Events is the subscription's buffered stream; it closes when the
// subscription ends for any reason.
func (sub *Subscription) Events() <-chan Event { return sub.s.ch }

// Err reports why Events ended. It is stable once the stream closed.
func (sub *Subscription) Err() error {
	sub.hub.mu.Lock()
	defer sub.hub.mu.Unlock()
	return sub.s.err
}

// Unsubscribe removes the subscription and closes its stream with a nil Err.
// It is idempotent and safe concurrently with Publish, Hub.Close, and being
// evicted.
func (sub *Subscription) Unsubscribe() {
	h := sub.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	if sub.s.closed {
		return
	}
	sub.s.closed = true
	set := h.rooms[sub.s.room]
	delete(set, sub.s)
	if len(set) == 0 {
		delete(h.rooms, sub.s.room)
	}
	close(sub.s.ch)
}
