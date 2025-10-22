package store

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rafian-git/valuefirst-assignment/internal/queue"
)

// TestStoreConcurrentApplyAndGet hammers Apply and Get from many goroutines.
// Run with -race to verify there are no data races.
func TestStoreConcurrentApplyAndGet(t *testing.T) {
	const (
		products   = 200
		writesPerP = 50
		readers    = 20
	)

	st := New()

	// writer goroutines
	var wg sync.WaitGroup
	wg.Add(products)
	for i := 0; i < products; i++ {
		id := fmt.Sprintf("p-%03d", i) // p-000, p-001, ...
		go func(pid string) {
			defer wg.Done()
			for v := 1; v <= writesPerP; v++ {
				price := float64(v)
				stock := v
				st.Apply(queue.UpdateEvent{ProductID: pid, Price: &price, Stock: &stock})
				// tiny pause to encourage interleaving
				if v%7 == 0 {
					time.Sleep(time.Microsecond)
				}
			}
		}(id)
	}

	// readers continuously read while writers are active
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var rg sync.WaitGroup //wait group for readers
	rg.Add(readers)
	for r := 0; r < readers; r++ {
		go func() {
			defer rg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// read a few random-ish keys (deterministic for test)
					for i := 0; i < 10; i++ {
						id := fmt.Sprintf("p-%03d", i)
						_, _ = st.Get(id) // existence not guaranteed early in the test
					}
				}
			}
		}()
	}

	wg.Wait() // all writes finished
	cancel()  // stop readers
	rg.Wait() // readers stopped

	// verify each product has its last write
	for i := 0; i < products; i++ {
		id := fmt.Sprintf("p-%03d", i)
		p, ok := st.Get(id)
		if !ok {
			t.Fatalf("expected product %s to exist", id)
		}
		if p.Price != float64(writesPerP) || p.Stock != writesPerP {
			t.Fatalf("product %s: want price=%v stock=%d, got price=%v stock=%d",
				id, float64(writesPerP), writesPerP, p.Price, p.Stock)
		}
	}
}

// TestStorePartialUpdates ensures that partial events only update specified fields.
func TestStorePartialUpdates(t *testing.T) {
	st := New()
	price := 10.0
	stock := 5
	st.Apply(queue.UpdateEvent{ProductID: "x", Price: &price, Stock: &stock})

	// Only price update
	price2 := 20.0
	st.Apply(queue.UpdateEvent{ProductID: "x", Price: &price2})
	p, ok := st.Get("x")
	if !ok {
		t.Fatalf("expected product x")
	}
	if p.Price != 20.0 || p.Stock != 5 {
		t.Fatalf("after price-only update, got price=%v stock=%d", p.Price, p.Stock)
	}

	// Only stock update
	stock2 := 7
	st.Apply(queue.UpdateEvent{ProductID: "x", Stock: &stock2})
	p, ok = st.Get("x")
	if !ok || p.Price != 20.0 || p.Stock != 7 {
		t.Fatalf("after stock-only update, got price=%v stock=%d", p.Price, p.Stock)
	}
}
