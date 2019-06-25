package gopool

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrorPoolClosed submits task into closed pool.
	ErrorPoolClosed = errors.New("gopool closed")
)

type task func()

// Pool go routines pool
type Pool struct {
	PanicHandler func(interface{})

	idleTimeout time.Duration

	pending chan task
	workers chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	mux    sync.RWMutex
	wg     sync.WaitGroup
	closed bool
}

// Do sumits task to pool.
func (p *Pool) Do(ctx context.Context, t task) error {
	p.mux.RLock()
	if p.closed {
		p.mux.RUnlock()
		return ErrorPoolClosed
	}
	p.mux.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.pending <- t:
	case p.workers <- struct{}{}:
		p.wg.Add(1)
		go p.loop(t)
	}
	return nil
}

// Close pool and wait all task finish.
func (p *Pool) Close() {
	if p.close() {
		p.wg.Wait()
	}
}

func (p *Pool) recover() {
	if h := p.PanicHandler; h != nil {
		if v := recover(); v != nil {
			h(v)
		}
	}
}

func (p *Pool) loop(t task) {
	defer func() {
		p.wg.Done()
		<-p.workers
	}()
	defer p.recover()

	timer := time.NewTimer(p.idleTimeout)
	defer timer.Stop()

	for {
		t()

		select {
		case <-timer.C:
			return
		case <-p.ctx.Done():
			return
		case t = <-p.pending:
			if t == nil {
				return
			}

			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(p.idleTimeout)
		}
	}
}

func (p *Pool) close() bool {
	p.mux.Lock()
	if p.closed {
		p.mux.Unlock()
		return false
	}

	p.closed = true
	p.mux.Unlock()

	close(p.pending)
	close(p.workers)
	p.cancel()
	return true
}

// New create goroutines pool
func New(workers int, idleTimeout time.Duration) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	return &Pool{
		idleTimeout: idleTimeout,
		ctx:         ctx,
		cancel:      cancel,
		pending:     make(chan task),
		workers:     make(chan struct{}, workers),
	}
}
