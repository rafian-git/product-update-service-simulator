package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rafian-git/valuefirst-assignment/internal/httpserver"
	"github.com/rafian-git/valuefirst-assignment/internal/queue"
	"github.com/rafian-git/valuefirst-assignment/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	port := getenv("PORT", "8080")
	workers := atoi(getenv("WORKERS", "4"), 4)
	qsize := atoi(getenv("QUEUE_SIZE", "128"), 128)

	q := queue.New(qsize)
	st := store.New()

	wg := q.StartWorkers(ctx, workers, func(ev queue.UpdateEvent) {
		st.Apply(ev)
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      httpserver.New(q, st),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("HTTP server listening on :%s (workers=%d, queue=%d)", port, workers, qsize)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	//log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	// drain queue (up to 10s)
	drainDeadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(drainDeadline) {
		if q.Len() == 0 {
			break
		} else {
			log.Printf("draining queue, %d items remaining...", q.Len())
		}
		time.Sleep(50 * time.Millisecond)
	}

	q.Close()
	wg.Wait()
	log.Println("graceful shutdown complete.\nExiting...")
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func atoi(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
