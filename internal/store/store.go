package store

import (
	"github.com/rafian-git/valuefirst-assignment/internal/models"
	"github.com/rafian-git/valuefirst-assignment/internal/queue"
	"sync"
)

type Store struct {
	mu   sync.RWMutex
	data map[string]models.Product
}

func New() *Store { return &Store{data: make(map[string]models.Product)} }

func (s *Store) Apply(ev queue.UpdateEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.data[ev.ProductID]
	p.ProductID = ev.ProductID
	if ev.Price != nil {
		p.Price = *ev.Price
	}
	if ev.Stock != nil {
		p.Stock = *ev.Stock
	}
	s.data[ev.ProductID] = p
}

func (s *Store) Get(id string) (models.Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, found := s.data[id]
	if !found {
		return models.Product{}, false
	}
	return p, found
}
