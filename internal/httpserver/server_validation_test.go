package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rafian-git/valuefirst-assignment/internal/queue"
	"github.com/rafian-git/valuefirst-assignment/internal/store"
)

func newServerForTest(t *testing.T) (http.Handler, func()) {
	t.Helper()
	q := queue.New(64)
	st := store.New()
	ctx, cancel := context.WithCancel(context.Background())
	q.StartWorkers(ctx, 2, func(ev queue.UpdateEvent) { st.Apply(ev) })
	return New(q, st), func() {
		cancel()
		q.Close()
	}
}

func TestHealthzOK(t *testing.T) {
	h, cleanup := newServerForTest(t)
	defer cleanup()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestPostEvent_ValidationErrors(t *testing.T) {
	h, cleanup := newServerForTest(t)
	defer cleanup()

	tests := []struct {
		name string
		body string
	}{
		{"bad json", `{"product_id":1`},
		{"missing product_id", `{"price":1}`},
		{"missing fields", `{"product_id":"x"}`},
		{"price out of range", `{"product_id":"x","price":-1}`},
		{"stock out of range", `{"product_id":"x","stock":-2}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(tc.body))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d; body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h, cleanup := newServerForTest(t)
	defer cleanup()

	// POST to /products should be 405
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/products/abc", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}

	// GET to /events should be 405
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/events", nil)
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr2.Code)
	}
}

func TestGetProduct_NotFound_And_BadRequest(t *testing.T) {
	h, cleanup := newServerForTest(t)
	defer cleanup()

	// missing id
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/products/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	// unknown id
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/products/does-not-exist", nil)
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr2.Code)
	}
}
