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

func TestPartialAndOverrideUpdates(t *testing.T) {
	q := queue.New(64)
	st := store.New()
	ctx, cancel := context.WithCancel(context.Background())
	q.StartWorkers(ctx, 2, func(ev queue.UpdateEvent) { st.Apply(ev) })
	h := New(q, st)
	defer func() { cancel(); q.Close() }()

	// price-only
	postJSON(t, h, `{"product_id":"abc","price":10}`)
	awaitProcessed()

	// verify
	price, stock := getPriceStock(t, h, "abc")
	if price != 10 || stock != 0 {
		t.Fatalf("after price-only: want price=10 stock=0, got price=%v stock=%d", price, stock)
	}
	// stock-only
	postJSON(t, h, `{"product_id":"abc","stock":5}`)
	awaitProcessed()

	price, stock = getPriceStock(t, h, "abc")
	if price != 10 || stock != 5 {
		t.Fatalf("after stock-only: want price=10 stock=5, got price=%v stock=%d", price, stock)
	}

	// update price
	postJSON(t, h, `{"product_id":"abc","price":20}`)
	awaitProcessed()

	price, stock = getPriceStock(t, h, "abc")
	if price != 20 || stock != 5 {
		t.Fatalf("after override: want price=20 stock=5, got price=%v stock=%d", price, stock)
	}
}

func postJSON(t *testing.T, h http.Handler, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("POST /events: expected 202, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func getPriceStock(t *testing.T, h http.Handler, id string) (float64, int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/products/"+id, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /products/%s: expected 200, got %d body=%s", id, rr.Code, rr.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	return got["price"].(float64), int(got["stock"].(float64))
}

func awaitProcessed() {
	time.Sleep(30 * time.Millisecond) // small delay to allow worker to apply
}
