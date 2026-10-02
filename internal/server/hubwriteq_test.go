package server

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// TestHubEvictingLastSubscriberDropsRoomEntry pins eviction's cleanup: the
// room's last subscriber evicted for slowness releases the room key with it,
// so a dead room key cannot linger in the hub's map.
func TestHubEvictingLastSubscriberDropsRoomEntry(t *testing.T) {
	h := NewHub()
	sub, err := h.Subscribe("room-a")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Fill the buffer without reading, then one more publish evicts.
	for range config.HubSubscriberBuffer + 1 {
		h.Publish("room-a", Event{Kind: EventKindMove, Payload: "D4"})
	}

	for range config.HubSubscriberBuffer {
		if _, open := <-sub.Events(); !open {
			t.Fatal("stream closed before the buffered events drained")
		}
	}
	if _, open := <-sub.Events(); open {
		t.Fatal("stream still open after eviction, want it closed")
	}
	if err := sub.Err(); !errors.Is(err, ErrSlowConsumer) {
		t.Errorf("stream error = %v, want ErrSlowConsumer", err)
	}

	h.mu.Lock()
	_, keyed := h.rooms["room-a"]
	setLen := len(h.rooms)
	h.mu.Unlock()
	if keyed || setLen != 0 {
		t.Errorf("hub keys after the last eviction = %d (room-a present %t), want the room entry dropped", setLen, keyed)
	}

	// Publishing to the dropped key stays a no-op and the key is reusable.
	h.Publish("room-a", Event{Kind: EventKindMove, Payload: "H8"})
	if again, err := h.Subscribe("room-a"); err != nil {
		t.Fatalf("resubscribe after eviction cleanup: %v", err)
	} else {
		again.Unsubscribe()
	}
}

// lockedBuffer is a log sink the test can poll while the queue worker
// writes: both sides take the mutex.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestWriteQueueNilErrorSinkLogsDefault pins the constructor's nil-sink
// fallback: a fire-and-forget failure reaches the standard logger, and the
// worker survives the report.
func TestWriteQueueNilErrorSinkLogsDefault(t *testing.T) {
	buf := &lockedBuffer{}
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	q := NewWriteQueue(nil)

	boom := errors.New("write queue probe failure")
	if err := q.SendAsync(func(context.Context) error { return boom }); err != nil {
		t.Fatalf("send async: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(buf.String(), "server: write queue async apply: "+boom.Error()) {
		if time.Now().After(deadline) {
			t.Fatalf("default sink logged %q, want the async apply failure", buf.String())
		}
		time.Sleep(time.Millisecond)
	}

	// The worker outlives the report: a later send applies and Close drains.
	if err := q.Send(func(context.Context) error { return nil }); err != nil {
		t.Fatalf("send after the async failure: %v", err)
	}
	q.Close()
}
