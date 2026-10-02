package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Bounded-wait budgets. Every wait below is event driven, the bounds only
// fail a wedged implementation. The engine scales such budgets with a
// build-tagged race factor in its own file; the M6a file list fixes these
// two files, so the bounds are generous enough to hold under -race directly.
const (
	// testBlockBound is the negative budget: a channel that must stay quiet.
	testBlockBound = 1 * time.Second
	// testWaitBound is the positive budget: a channel that must deliver.
	testWaitBound = 10 * time.Second
)

func recvWithin[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(testWaitBound):
		t.Fatalf("%s did not complete within %v", what, testWaitBound)
	}
	var zero T
	return zero
}

func assertBlocked[T any](t *testing.T, ch <-chan T, what string) {
	t.Helper()
	select {
	case v := <-ch:
		t.Fatalf("%s completed unexpectedly with %v", what, v)
	case <-time.After(testBlockBound):
	}
}

// TestWriteQueueSerialOrderManyProducers proves the worker applies strictly
// serially: applied and count are plain non-atomic writes touched only
// inside mutations, so the race detector flags any second applier, and the
// per-producer subsequence must stay FIFO for sync and async senders alike.
func TestWriteQueueSerialOrderManyProducers(t *testing.T) {
	const producers, perProducer = 8, 500
	type rec struct{ producer, seq int }
	applied := make([]rec, 0, producers*perProducer)
	var count int

	q := NewWriteQueue(nil)
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for s := 0; s < perProducer; s++ {
				mut := func(context.Context) error {
					applied = append(applied, rec{p, s})
					count++
					return nil
				}
				var err error
				if p%2 == 0 {
					err = q.Send(mut)
				} else {
					err = q.SendAsync(mut)
				}
				if err != nil {
					t.Errorf("producer %d seq %d send: %v", p, s, err)
					return
				}
			}
		}(p)
	}
	wg.Wait()
	q.Close()

	if got, want := count, producers*perProducer; got != want {
		t.Fatalf("non-atomic applied counter = %d, want %d", got, want)
	}
	if got, want := len(applied), producers*perProducer; got != want {
		t.Fatalf("applied log length = %d, want %d", got, want)
	}
	next := make([]int, producers)
	for i, r := range applied {
		if r.seq != next[r.producer] {
			t.Fatalf("applied[%d] = producer %d seq %d, want seq %d (per-producer FIFO broken)", i, r.producer, r.seq, next[r.producer])
		}
		next[r.producer]++
	}
	for p, n := range next {
		if n != perProducer {
			t.Fatalf("producer %d applied %d of %d mutations", p, n, perProducer)
		}
	}
}

func TestWriteQueueFullBlocksThenUnblocks(t *testing.T) {
	var applied int
	release := make(chan struct{})
	q := NewWriteQueue(nil)

	if err := q.SendAsync(func(context.Context) error {
		<-release
		applied++
		return nil
	}); err != nil {
		t.Fatalf("gate send: %v", err)
	}
	for i := 0; i < config.WriteQueueDepth; i++ {
		if err := q.SendAsync(func(context.Context) error { applied++; return nil }); err != nil {
			t.Fatalf("fill send %d: %v", i, err)
		}
	}

	sDone := make(chan error, 1)
	go func() {
		sDone <- q.Send(func(context.Context) error { applied++; return nil })
	}()
	assertBlocked(t, sDone, "Send on a full queue")

	close(release)
	if err := recvWithin(t, sDone, "Send after space freed"); err != nil {
		t.Fatalf("Send after unblock: %v", err)
	}
	q.Close()
	if got, want := applied, config.WriteQueueDepth+2; got != want {
		t.Fatalf("applied = %d, want %d (gate + fill + unblocked sender)", got, want)
	}
}

func TestWriteQueueSyncErrorDelivery(t *testing.T) {
	boom := errors.New("boom-sync")
	q := NewWriteQueue(nil)
	err := q.Send(func(context.Context) error { return boom })
	q.Close()
	if !errors.Is(err, boom) {
		t.Fatalf("synchronous Send error = %v, want %v", err, boom)
	}
}

func TestWriteQueueAsyncErrorSink(t *testing.T) {
	boom := errors.New("boom-async")
	sink := make(chan error, 1)
	q := NewWriteQueue(func(err error) { sink <- err })
	if err := q.SendAsync(func(context.Context) error { return boom }); err != nil {
		t.Fatalf("SendAsync: %v", err)
	}
	if got := recvWithin(t, sink, "error sink delivery"); !errors.Is(got, boom) {
		t.Fatalf("sink error = %v, want %v", got, boom)
	}
	q.Close()
}

func TestWriteQueueCloseDrainsEverythingQueued(t *testing.T) {
	const queued = 300 // above WriteQueueDepth, so fillers re-block and drain
	var applied int
	q := NewWriteQueue(nil)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < queued/4; i++ {
				if err := q.SendAsync(func(context.Context) error { applied++; return nil }); err != nil {
					t.Errorf("send %d: %v", i, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	q.Close()
	if got, want := applied, queued; got != want {
		t.Fatalf("applied after Close = %d, want %d (drain must apply every queued mutation)", got, want)
	}
}

func TestWriteQueueCloseIdempotentAndSendAfterCloseRejected(t *testing.T) {
	var applied int
	q := NewWriteQueue(nil)
	if err := q.Send(func(context.Context) error { applied++; return nil }); err != nil {
		t.Fatalf("Send: %v", err)
	}
	q.Close()
	q.Close()
	if err := q.Send(func(context.Context) error { applied++; return nil }); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("Send after Close = %v, want ErrQueueClosed", err)
	}
	if err := q.SendAsync(func(context.Context) error { applied++; return nil }); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("SendAsync after Close = %v, want ErrQueueClosed", err)
	}
	if applied != 1 {
		t.Fatalf("applied = %d, want 1 (rejected mutations must not apply)", applied)
	}
}

// TestWriteQueueBlockedSenderDuringClosePolicy pins the Close policy: a
// sender blocked on a full queue unblocks with ErrQueueClosed the moment
// Close decides, without waiting for the drain; Close itself returns only
// after the worker applied every already-queued mutation.
func TestWriteQueueBlockedSenderDuringClosePolicy(t *testing.T) {
	var applied int
	release := make(chan struct{})
	q := NewWriteQueue(nil)
	if err := q.SendAsync(func(context.Context) error {
		<-release
		applied++
		return nil
	}); err != nil {
		t.Fatalf("gate send: %v", err)
	}
	for i := 0; i < config.WriteQueueDepth; i++ {
		if err := q.SendAsync(func(context.Context) error { applied++; return nil }); err != nil {
			t.Fatalf("fill send %d: %v", i, err)
		}
	}

	sDone := make(chan error, 1)
	go func() {
		sDone <- q.Send(func(context.Context) error { applied++; return nil })
	}()
	assertBlocked(t, sDone, "Send on a full queue")

	closeDone := make(chan struct{})
	go func() {
		q.Close()
		close(closeDone)
	}()

	if err := recvWithin(t, sDone, "blocked Send during Close"); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("blocked Send during Close = %v, want ErrQueueClosed", err)
	}
	assertBlocked(t, closeDone, "Close while the worker is still draining the gated queue")
	close(release)
	recvWithin(t, closeDone, "Close after the drain")
	if got, want := applied, config.WriteQueueDepth+1; got != want {
		t.Fatalf("applied = %d, want %d (gate + every queued mutation, rejected sender excluded)", got, want)
	}
	q.Close()
}

func TestWriteQueueApplyTimeoutCutsWedgedMutation(t *testing.T) {
	q := NewWriteQueue(nil)
	q.ApplyTimeout = 20 * time.Millisecond
	err := q.Send(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	q.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wedged mutation result = %v, want context.DeadlineExceeded", err)
	}
}

// TestWriteQueueStressConcurrentClose hammers Send and SendAsync from many
// producers while Close runs concurrently. Whatever the interleaving, every
// accepted mutation applies exactly once in per-producer FIFO order, every
// rejection is ErrQueueClosed, and the sink stays silent.
func TestWriteQueueStressConcurrentClose(t *testing.T) {
	const producers, perProducer = 8, 400
	type rec struct{ producer, seq int }
	applied := make([]rec, 0, producers*perProducer)
	sink := make(chan error, 1)
	q := NewWriteQueue(func(err error) { sink <- err })

	start := make(chan struct{})
	var wg sync.WaitGroup
	accepted := make([]chan int, producers)
	for p := 0; p < producers; p++ {
		accepted[p] = make(chan int, 1)
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			<-start
			ok := 0
			for s := 0; s < perProducer; s++ {
				var err error
				if p%2 == 0 {
					err = q.Send(func(context.Context) error {
						applied = append(applied, rec{p, s})
						return nil
					})
				} else {
					err = q.SendAsync(func(context.Context) error {
						applied = append(applied, rec{p, s})
						return nil
					})
				}
				if err != nil {
					if !errors.Is(err, ErrQueueClosed) {
						t.Errorf("producer %d seq %d: %v", p, s, err)
					}
					break
				}
				ok++
			}
			accepted[p] <- ok
		}(p)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		q.Close()
	}()
	close(start)
	wg.Wait()

	total := 0
	for p, ch := range accepted {
		n := recvWithin(t, ch, "producer acceptance report")
		if n > perProducer {
			t.Fatalf("producer %d accepted %d, cap %d", p, n, perProducer)
		}
		total += n
	}
	if len(applied) != total {
		t.Fatalf("applied %d mutations, want %d accepted", len(applied), total)
	}
	next := make([]int, producers)
	for i, r := range applied {
		if r.seq != next[r.producer] {
			t.Fatalf("applied[%d] = producer %d seq %d, want seq %d", i, r.producer, r.seq, next[r.producer])
		}
		next[r.producer]++
	}
	select {
	case err := <-sink:
		t.Fatalf("error sink hit: %v", err)
	default:
	}
}
