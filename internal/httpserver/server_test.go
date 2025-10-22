package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rafian-git/valuefirst-assignment/internal/queue"
	"github.com/rafian-git/valuefirst-assignment/internal/store"
)

func newTestServer(t *testing.T) (*queue.Queue, *store.Store, http.Handler, context.CancelFunc) {
	t.Helper()
	q := queue.New(64)
	st := store.New()
	ctx, cancel := context.WithCancel(context.Background())
	q.StartWorkers(ctx, 2, func(ev queue.UpdateEvent) { st.Apply(ev) })
	return q, st, New(q, st), cancel
}

func TestPostAndGet(t *testing.T) {
	q, _, h, cancel := newTestServer(t)
	defer func() { q.Close(); cancel() }()

	body := []byte(`{"product_id":"abc","price":12.5,"stock":7}`)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	time.Sleep(50 * time.Millisecond)

	req2 := httptest.NewRequest(http.MethodGet, "/products/abc", nil)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr2.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr2.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if got["price"].(float64) != 12.5 || int(got["stock"].(float64)) != 7 {
		t.Fatalf("unexpected body: %v", got)
	}
}
