package gopool

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
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

func BenchmarkMain(b *testing.B) {
	idle := 5 * time.Second

	testTask := func() {
		n := rand.Int63n(50)
		time.Sleep(time.Duration(n) * time.Microsecond)
	}

	for _, workers := range []int{10, 50, 100, 500, 1000} {
		b.Run(fmt.Sprintf("worker %d", workers), func(b *testing.B) {
			b.Run("pool", func(b *testing.B) {
				p := New(workers, idle)

				b.StartTimer()
				for i := 0; i < b.N; i++ {
					p.Do(context.Background(), testTask)
				}
				p.Close()
				b.StopTimer()
			})

			b.Run("fan out", func(b *testing.B) {
				var wg sync.WaitGroup
				tasks := make(chan func())

				for i := 0; i < workers; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()

						for task := range tasks {
							task()
						}
					}()
				}

				b.StartTimer()
				for i := 0; i < b.N; i++ {
					tasks <- testTask
				}
				close(tasks)
				wg.Wait()
				b.StopTimer()
			})

			b.Run("token bucket", func(b *testing.B) {
				tokens := make(chan struct{}, workers)
				tasks := make(chan task)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer wg.Done()

					for task := range tasks {
						tokens <- struct{}{}

						go func(task func()) {
							task()
							<-tokens
						}(task)
					}
				}()

				b.StartTimer()
				for i := 0; i < b.N; i++ {
					tasks <- testTask
				}
				close(tasks)
				wg.Wait()
				b.StopTimer()
			})
		})
	}
}
