package server

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Mutation is one unit of persistent state change. The queue worker calls
// it serially under a fresh context carrying WriteQueue.ApplyTimeout as the
// deadline, so one wedged write is cut off instead of stalling every later
// mutation behind it.
type Mutation func(ctx context.Context) error

// ErrQueueClosed rejects a mutation the queue can no longer serve: the Close
// policy set the closed flag before the sender enqueued, which includes a
// sender blocked on a full queue at that moment.
var ErrQueueClosed = errors.New("server: write queue closed")

// defaultApplyTimeout is the per-mutation deadline fallback. Config carries
// no such constant yet, so the constructor owns the default (config gap,
// reported with the milestone).
const defaultApplyTimeout = 30 * time.Second

type mutation struct {
	apply Mutation
	done  chan error // nil for fire-and-forget
}

// WriteQueue is the single-writer mutation queue every database write flows
// through: SQLite takes one writer at a time, so persistence serializes here
// instead of inside the driver. One worker goroutine applies queued
// mutations strictly in FIFO order. A full queue blocks producers, it never
// drops: gameplay may slow but no write is lost.
type WriteQueue struct {
	// ApplyTimeout bounds each mutation's context. It is a wedged-write
	// ceiling, not a performance bound.
	ApplyTimeout time.Duration

	mu         sync.Mutex
	notEmpty   *sync.Cond
	notFull    *sync.Cond
	pending    []mutation
	closed     bool
	closeOnce  sync.Once
	onError    func(error)
	workerDone chan struct{}
}

// NewWriteQueue starts the worker of a queue with config.WriteQueueDepth
// slots. onError receives the apply error of every fire-and-forget mutation
// that fails; nil falls back to the standard logger.
func NewWriteQueue(onError func(error)) *WriteQueue {
	q := &WriteQueue{
		ApplyTimeout: defaultApplyTimeout,
		pending:      make([]mutation, 0, config.WriteQueueDepth),
		onError:      onError,
		workerDone:   make(chan struct{}),
	}
	if q.onError == nil {
		q.onError = func(err error) { log.Printf("server: write queue async apply: %v", err) }
	}
	q.notEmpty = sync.NewCond(&q.mu)
	q.notFull = sync.NewCond(&q.mu)
	go q.work()
	return q
}

// Send enqueues m and waits for the worker to apply it, returning the apply
// result. It blocks while the queue already holds config.WriteQueueDepth
// pending mutations.
func (q *WriteQueue) Send(m Mutation) error {
	done := make(chan error, 1)
	if err := q.enqueue(mutation{apply: m, done: done}); err != nil {
		return err
	}
	return <-done
}

// SendAsync enqueues m fire-and-forget: an apply error goes to the
// constructor's error sink instead of the caller.
func (q *WriteQueue) SendAsync(m Mutation) error {
	return q.enqueue(mutation{apply: m})
}

func (q *WriteQueue) enqueue(m mutation) error {
	q.mu.Lock()
	for len(q.pending) == config.WriteQueueDepth && !q.closed {
		q.notFull.Wait()
	}
	if q.closed {
		q.mu.Unlock()
		return ErrQueueClosed
	}
	q.pending = append(q.pending, m)
	q.notEmpty.Signal()
	q.mu.Unlock()
	return nil
}

func (q *WriteQueue) work() {
	defer close(q.workerDone)
	for {
		q.mu.Lock()
		for len(q.pending) == 0 && !q.closed {
			q.notEmpty.Wait()
		}
		if len(q.pending) == 0 {
			q.mu.Unlock()
			return
		}
		m := q.pending[0]
		q.pending = q.pending[1:]
		q.notFull.Signal()
		q.mu.Unlock()
		q.applyOne(m)
	}
}

func (q *WriteQueue) applyOne(m mutation) {
	ctx, cancel := context.WithTimeout(context.Background(), q.ApplyTimeout)
	err := m.apply(ctx)
	cancel()
	if m.done != nil {
		m.done <- err
		return
	}
	if err != nil {
		q.onError(err)
	}
}

// Close stops accepting mutations and returns after the worker applied
// everything already enqueued. It is idempotent and safe to call
// concurrently with Send.
//
// Close policy, deterministic under any interleaving: a Send or SendAsync
// returns ErrQueueClosed exactly when the closed flag was set before that
// call appended its mutation, so a sender blocked on a full queue unblocks
// with ErrQueueClosed as soon as Close decides, without waiting for the
// drain; everything appended before the flag applies before the worker
// exits.
func (q *WriteQueue) Close() {
	q.closeOnce.Do(func() {
		q.mu.Lock()
		q.closed = true
		q.notFull.Broadcast()
		q.notEmpty.Signal()
		q.mu.Unlock()
	})
	<-q.workerDone
}
