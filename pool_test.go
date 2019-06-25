package gopool

import (
	"context"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_Do(t *testing.T) {
	workers := 100
	tasks := workers * 100
	p := New(100, 10*time.Second)

	var n int32
	for i := 0; i < tasks; i++ {
		err := p.Do(context.Background(), func() {
			time.Sleep(100 * time.Microsecond)
			atomic.AddInt32(&n, 1)
		})

		if err != nil {
			t.Fatal("should not error")
		}
	}

	p.Close()
	if n != int32(tasks) {
		t.Fatalf("worker execute, except: %d, actual: %d", workers*10, n)
	}
}

func TestPool_DoWait(t *testing.T) {
	p := New(2, 10*time.Second)
	p.Do(context.Background(), func() {
		time.Sleep(200 * time.Microsecond)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Microsecond)
	defer cancel()

	err := p.Do(ctx, func() {
		time.Sleep(200 * time.Microsecond)
	})
	if err != nil {
		t.Fatal("should not error")
	}

	err = p.Do(ctx, func() {})
	if err != context.DeadlineExceeded {
		t.Fatalf("should return context.DeadlineExceeded error")
	}
}

func TestPool_DoError(t *testing.T) {
	t.Run("panic", func(t *testing.T) {
		t.Parallel()

		ok := false
		p := New(2, 1*time.Second)
		p.PanicHandler = func(v interface{}) {
			ok = true
		}

		p.Do(context.Background(), func() {
			panic("error")
		})
		p.Close()

		if !ok {
			t.Fatal("panic handler not work")
		}
	})

	t.Run("closed", func(t *testing.T) {
		t.Parallel()

		p := New(1, 1*time.Second)
		p.Close()

		err := p.Do(context.Background(), func() {})
		if err != ErrorPoolClosed {
			t.Fatalf("should return pool closed error")
		}
	})
}

func BenchmarkPool(b *testing.B) {
	p := New(100, 5*time.Second)
	for i := 0; i < b.N; i++ {
		p.Do(context.Background(), func() {
			n := rand.Int63n(50)
			time.Sleep(time.Duration(n) * time.Microsecond)
		})
	}
	p.Close()
}
