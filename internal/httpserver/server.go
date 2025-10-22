package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rafian-git/valuefirst-assignment/internal/queue"
	"github.com/rafian-git/valuefirst-assignment/internal/store"
)

type Server struct {
	q  *queue.Queue
	st *store.Store
}

func New(q *queue.Queue, st *store.Store) http.Handler {
	s := &Server{q: q, st: st}
	mux := http.NewServeMux()
	mux.HandleFunc("/events", s.handlePostEvent)
	mux.HandleFunc("/products/", s.handleGetProduct)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("ok"))
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	})
	return logging(mux)
}

type postEventDTO struct {
	ProductID string   `json:"product_id"`
	Price     *float64 `json:"price,omitempty"`
	Stock     *int     `json:"stock,omitempty"`
}

func (s *Server) handlePostEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var dto postEventDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := validate(dto); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.q.Enqueue(queue.UpdateEvent{ProductID: dto.ProductID, Price: dto.Price, Stock: dto.Stock})
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("queued"))
}

func validate(dto postEventDTO) error {
	if strings.TrimSpace(dto.ProductID) == "" {
		return errors.New("product_id is required")
	}
	if dto.Price == nil && dto.Stock == nil {
		return errors.New("at least one of price or stock is required")
	}
	if dto.Price != nil && (*dto.Price < 0 || *dto.Price > 1e9) {
		return fmt.Errorf("price out of range: %f", *dto.Price)
	}
	if dto.Stock != nil && (*dto.Stock < 0 || *dto.Stock > 1_000_000_000) {
		return fmt.Errorf("stock out of range: %d", *dto.Stock)
	}
	return nil
}

func (s *Server) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/products/")
	if id == "" {
		http.Error(w, "missing product id", http.StatusBadRequest)
		return
	}
	p, found := s.st.Get(id)
	if !found {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
