package queue

import (
	"context"
	"log"
	"sync"
)

type UpdateEvent struct {
	ProductID string
	Price     *float64
	Stock     *int
}

type Queue struct {
	ch chan UpdateEvent
}

func New(buf int) *Queue { return &Queue{ch: make(chan UpdateEvent, buf)} }

func (q *Queue) Enqueue(ev UpdateEvent) { q.ch <- ev }

func (q *Queue) Len() int { return len(q.ch) }

func (q *Queue) Close() { close(q.ch) }

func (q *Queue) StartWorkers(ctx context.Context, n int, apply func(UpdateEvent)) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		workerNumber := i + 1
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					log.Printf("context canceled, worker-%d stopping", workerNumber)
					return
				case ev, ok := <-q.ch:
					if !ok {
						log.Printf("queue closed, worker-%d stopping", workerNumber)
						return
					}
					apply(ev)
				}
			}
		}()
	}
	return &wg
}
