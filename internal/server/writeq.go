package server

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type Mutation func(ctx context.Context) error

var ErrQueueClosed = errors.New("server: write queue closed")

type mutation struct {
	apply Mutation
	done  chan error
}

type WriteQueue struct {
	ApplyTimeout time.Duration

	pending    []mutation
	mu         sync.Mutex
	notEmpty   *sync.Cond
	notFull    *sync.Cond
	closed     bool
	closeOnce  sync.Once
	onError    func(error)
	workerDone chan struct{}
}

func NewWriteQueue(onError func(error)) *WriteQueue {
	q := &WriteQueue{
		ApplyTimeout: time.Duration(config.WriteQueueApplyTimeoutMs) * time.Millisecond,
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

func (q *WriteQueue) Send(m Mutation) error {
	done := make(chan error, 1)
	if err := q.enqueue(mutation{apply: m, done: done}); err != nil {
		return err
	}
	return <-done
}

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
