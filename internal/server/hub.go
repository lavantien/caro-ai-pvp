package server

import (
	"errors"
	"sync"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type Event struct {
	Kind    string
	Payload string
	Stat    *MoveStat
}

var (
	ErrHubClosed    = errors.New("server: hub closed")
	ErrSlowConsumer = errors.New("server: hub subscriber too slow, evicted")
)

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

type Subscription struct {
	hub *Hub
	s   *subscriber
}

func (sub *Subscription) Events() <-chan Event { return sub.s.ch }

func (sub *Subscription) Err() error {
	sub.hub.mu.Lock()
	defer sub.hub.mu.Unlock()
	return sub.s.err
}

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
