package server

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// testBurst is the publish pacing unit: every burst stays below
// config.HubSubscriberBuffer, so a subscriber that has not run at all during
// a burst still cannot be evicted. The drains between bursts are the
// synchronization, no sleeps.
const testBurst = 48

func recvEvent(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("stream closed while more events were expected")
		}
		return ev
	case <-time.After(testWaitBound):
		t.Fatal("stream stalled waiting for an event")
	}
	return Event{}
}

func expectEvent(t *testing.T, ch <-chan Event, payload string) {
	t.Helper()
	if ev := recvEvent(t, ch); ev.Payload != payload {
		t.Fatalf("event = %q, want %q", ev.Payload, payload)
	}
}

func expectClosed(t *testing.T, ch <-chan Event) {
	t.Helper()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("stream still open, want closed")
		}
	case <-time.After(testWaitBound):
		t.Fatal("stream not closed")
	}
}

// publishBurst publishes n events starting at sequence start, then drains
// every stream, checking each subscriber received the burst exactly once,
// in order.
func publishBurst(t *testing.T, h *Hub, room string, subs []*Subscription, n, start int) {
	t.Helper()
	for j := 0; j < n; j++ {
		h.Publish(room, Event{Kind: "board", Payload: strconv.Itoa(start + j)})
	}
	for _, s := range subs {
		for j := 0; j < n; j++ {
			expectEvent(t, s.Events(), strconv.Itoa(start+j))
		}
	}
}

func TestHubFanOutIdenticalOrder(t *testing.T) {
	h := NewHub()
	const subscribers, rounds = 5, 6
	subs := make([]*Subscription, subscribers)
	for i := range subs {
		s, err := h.Subscribe("room")
		if err != nil {
			t.Fatalf("subscribe %d: %v", i, err)
		}
		subs[i] = s
	}
	for r := 0; r < rounds; r++ {
		publishBurst(t, h, "room", subs, testBurst, 0)
	}
	h.Close()
}

// TestHubEvictsSlowSubscriberWhenFull pins the eviction point: buffer slots
// 0..N-1 all land, the slot-N publish finds the buffer full, evicts, and
// closes the stream with ErrSlowConsumer while fast subscribers keep going.
func TestHubEvictsSlowSubscriberWhenFull(t *testing.T) {
	h := NewHub()
	slow, err := h.Subscribe("room")
	if err != nil {
		t.Fatalf("slow subscribe: %v", err)
	}
	const fast = 2
	fastSubs := make([]*Subscription, fast)
	for i := range fastSubs {
		s, err := h.Subscribe("room")
		if err != nil {
			t.Fatalf("fast subscribe %d: %v", i, err)
		}
		fastSubs[i] = s
	}

	// Bursts fill the slow buffer to exactly config.HubSubscriberBuffer
	// while the fast streams drain empty after each one.
	drained := 0
	for drained+testBurst <= config.HubSubscriberBuffer {
		publishBurst(t, h, "room", fastSubs, testBurst, drained)
		drained += testBurst
	}
	published := drained
	for ; published < config.HubSubscriberBuffer; published++ {
		h.Publish("room", Event{Kind: "board", Payload: strconv.Itoa(published)})
	}
	for _, s := range fastSubs {
		for j := drained; j < published; j++ {
			expectEvent(t, s.Events(), strconv.Itoa(j))
		}
	}
	if err := slow.Err(); err != nil {
		t.Fatalf("slow subscriber evicted before its buffer filled: %v", err)
	}

	// The eviction trigger: the publish that finds the slow buffer full.
	h.Publish("room", Event{Kind: "board", Payload: strconv.Itoa(published)})
	published++
	if err := slow.Err(); !errors.Is(err, ErrSlowConsumer) {
		t.Fatalf("slow subscriber Err = %v, want ErrSlowConsumer", err)
	}

	const extra = 8
	for i := 1; i < extra; i++ {
		h.Publish("room", Event{Kind: "board", Payload: strconv.Itoa(published)})
		published++
	}
	for _, s := range fastSubs {
		for j := config.HubSubscriberBuffer; j < published; j++ {
			expectEvent(t, s.Events(), strconv.Itoa(j))
		}
	}

	// The slow stream holds exactly its buffer, in order, then the close.
	for j := 0; j < config.HubSubscriberBuffer; j++ {
		expectEvent(t, slow.Events(), strconv.Itoa(j))
	}
	expectClosed(t, slow.Events())
	h.Close()
}

// TestHubUnsubscribeStopsDelivery pins the voluntary path deterministically:
// a nil Err, an idempotent call, and no event after the Unsubscribe lands.
func TestHubUnsubscribeStopsDelivery(t *testing.T) {
	h := NewHub()
	s, err := h.Subscribe("room")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	for j := 0; j < 3; j++ {
		h.Publish("room", Event{Kind: "board", Payload: strconv.Itoa(j)})
	}
	s.Unsubscribe()
	s.Unsubscribe()
	h.Publish("room", Event{Kind: "board", Payload: "after"})
	for j := 0; j < 3; j++ {
		expectEvent(t, s.Events(), strconv.Itoa(j))
	}
	expectClosed(t, s.Events())
	if err := s.Err(); err != nil {
		t.Fatalf("voluntary Unsubscribe Err = %v, want nil", err)
	}
	h.Close()
}

// TestHubUnsubscribeDuringPublish races an Unsubscribe spammer against a
// live publisher. The victim stops reading, so eviction is a legal second
// end state: both terminators are identifiable, the stream always closes
// with a contiguous ordered prefix, and the keeper stream is unaffected.
func TestHubUnsubscribeDuringPublish(t *testing.T) {
	h := NewHub()
	victim, err := h.Subscribe("room")
	if err != nil {
		t.Fatalf("victim subscribe: %v", err)
	}
	keeper, err := h.Subscribe("room")
	if err != nil {
		t.Fatalf("keeper subscribe: %v", err)
	}

	pubDone := make(chan struct{})
	unsubDone := make(chan struct{})
	go func() {
		defer close(unsubDone)
		for {
			select {
			case <-pubDone:
				return
			default:
				victim.Unsubscribe()
			}
		}
	}()

	const rounds = 8
	total := 0
	for r := 0; r < rounds; r++ {
		publishBurst(t, h, "room", []*Subscription{keeper}, testBurst, total)
		total += testBurst
	}
	close(pubDone)
	<-unsubDone

	// The keeper saw the whole stream, in order, despite the concurrent
	// churn on the victim.
	n := 0
	for {
		ev, ok := <-victim.Events()
		if !ok {
			break
		}
		if ev.Payload != strconv.Itoa(n) {
			t.Fatalf("victim event %d = %q, want a contiguous ordered prefix", n, ev.Payload)
		}
		n++
	}
	switch err := victim.Err(); {
	case err == nil:
	case errors.Is(err, ErrSlowConsumer):
	default:
		t.Fatalf("victim Err = %v, want nil or ErrSlowConsumer", err)
	}
	h.Close()
}

func TestHubCloseClosesAllSubscribers(t *testing.T) {
	h := NewHub()
	subs := make([]*Subscription, 0, 4)
	for _, room := range []string{"a", "b"} {
		for i := 0; i < 2; i++ {
			s, err := h.Subscribe(room)
			if err != nil {
				t.Fatalf("subscribe %s %d: %v", room, i, err)
			}
			subs = append(subs, s)
		}
	}
	h.Close()
	for i, s := range subs {
		if err := s.Err(); !errors.Is(err, ErrHubClosed) {
			t.Fatalf("subscriber %d Err = %v, want ErrHubClosed", i, err)
		}
		expectClosed(t, s.Events())
	}
	if _, err := h.Subscribe("a"); !errors.Is(err, ErrHubClosed) {
		t.Fatalf("Subscribe after Close = %v, want ErrHubClosed", err)
	}
	h.Close()
	h.Publish("a", Event{Kind: "board"})
}

func TestHubPublishWithoutSubscribersIsNoOp(t *testing.T) {
	h := NewHub()
	h.Publish("ghost", Event{Kind: "board", Payload: "before"})
	h.Publish("ghost", Event{Kind: "board", Payload: "before"})
	s, err := h.Subscribe("ghost")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	h.Publish("ghost", Event{Kind: "board", Payload: "after"})
	expectEvent(t, s.Events(), "after")
	s.Unsubscribe()
	expectClosed(t, s.Events())
	h.Close()
}

// TestHubConcurrentPublishersPreservePerPublisherOrder publishes to one
// room from many goroutines, round by round: each subscriber must see one
// identical global order, and inside it every publisher's own subsequence
// stays FIFO.
func TestHubConcurrentPublishersPreservePerPublisherOrder(t *testing.T) {
	h := NewHub()
	const publishers, subscribers, rounds, chunk = 4, 3, 5, 15
	perPublisher := rounds * chunk
	subs := make([]*Subscription, subscribers)
	for i := range subs {
		s, err := h.Subscribe("room")
		if err != nil {
			t.Fatalf("subscribe %d: %v", i, err)
		}
		subs[i] = s
	}

	streams := make([][]Event, subscribers)
	for r := 0; r < rounds; r++ {
		var wg sync.WaitGroup
		for p := 0; p < publishers; p++ {
			wg.Add(1)
			go func(p int) {
				defer wg.Done()
				for c := 0; c < chunk; c++ {
					h.Publish("room", Event{Kind: strconv.Itoa(p), Payload: strconv.Itoa(r*chunk + c)})
				}
			}(p)
		}
		wg.Wait()
		for i, s := range subs {
			for j := 0; j < publishers*chunk; j++ {
				streams[i] = append(streams[i], recvEvent(t, s.Events()))
			}
		}
	}
	h.Close()

	for i, st := range streams {
		next := make([]int, publishers)
		for j, ev := range st {
			p, err := strconv.Atoi(ev.Kind)
			if err != nil {
				t.Fatalf("subscriber %d event %d kind %q: %v", i, j, ev.Kind, err)
			}
			s, err := strconv.Atoi(ev.Payload)
			if err != nil {
				t.Fatalf("subscriber %d event %d payload %q: %v", i, j, ev.Payload, err)
			}
			if s != next[p] {
				t.Fatalf("subscriber %d event %d = publisher %d seq %d, want seq %d (per-publisher FIFO broken)", i, j, p, s, next[p])
			}
			next[p]++
		}
		for p, n := range next {
			if n != perPublisher {
				t.Fatalf("subscriber %d saw %d of %d events of publisher %d", i, n, perPublisher, p)
			}
		}
		if i > 0 {
			for j := range st {
				if st[j] != streams[0][j] {
					t.Fatalf("subscriber %d diverged from subscriber 0 at event %d: %+v vs %+v", i, j, st[j], streams[0][j])
				}
			}
		}
	}
}
