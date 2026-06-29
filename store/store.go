package store

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"ticket-api/models"
)

// Sentinel errors let callers check the error type without string matching.
var (
	ErrNotFound     = errors.New("ticket not found")
	ErrNoSeats      = errors.New("not enough seats available")
	ErrTicketClosed = errors.New("ticket is closed")
)

// Store is a thread-safe, in-memory ticket store.
//
// sync.RWMutex is used instead of sync.Mutex so that concurrent reads never
// block each other — only a write blocks (and is blocked by) other accesses.
// This matters once traffic picks up.
type Store struct {
	mu      sync.RWMutex
	tickets map[int64]*models.Ticket
	nextID  atomic.Int64 // lock-free counter; safe across goroutines
}

// New returns an initialised store pre-loaded with seed data.
func New() *Store {
	s := &Store{tickets: make(map[int64]*models.Ticket)}
	s.seed()
	return s
}

func (s *Store) seed() {
	for _, r := range []models.CreateRequest{
		{Event: "ETH Lagos", Description: "Ethereum developer conference in Lagos", Price: 25.00, TotalSeats: 200},
		{Event: "Solana Hackathon", Description: "48-hour Solana build sprint", Price: 0.00, TotalSeats: 80},
		{Event: "Web3 Summit Abuja", Description: "Pan-African Web3 builders summit", Price: 15.00, TotalSeats: 500},
	} {
		s.Create(r)
	}
}

// Create inserts a new ticket and returns a copy of it.
func (s *Store) Create(req models.CreateRequest) *models.Ticket {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID.Add(1)
	now := time.Now().UTC()
	t := &models.Ticket{
		ID:             id,
		Event:          req.Event,
		Description:    req.Description,
		Price:          req.Price,
		TotalSeats:     req.TotalSeats,
		AvailableSeats: req.TotalSeats,
		Status:         models.StatusOpen,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.tickets[id] = t
	return clone(t)
}

// GetAll returns all tickets, optionally filtered by status.
// Passing an empty string returns everything.
func (s *Store) GetAll(status models.Status) []models.Ticket {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.Ticket, 0, len(s.tickets))
	for _, t := range s.tickets {
		if status == "" || t.Status == status {
			out = append(out, *t)
		}
	}
	return out
}

// GetByID returns a copy of the ticket or ErrNotFound.
func (s *Store) GetByID(id int64) (*models.Ticket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tickets[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(t), nil
}

// Update applies only the non-nil fields from req, leaving others unchanged.
func (s *Store) Update(id int64, req models.UpdateRequest) (*models.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tickets[id]
	if !ok {
		return nil, ErrNotFound
	}
	if req.Event != nil {
		t.Event = *req.Event
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.Price != nil {
		t.Price = *req.Price
	}
	if req.Status != nil {
		t.Status = *req.Status
	}
	t.UpdatedAt = time.Now().UTC()
	return clone(t), nil
}

// Delete removes a ticket or returns ErrNotFound.
func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tickets[id]; !ok {
		return ErrNotFound
	}
	delete(s.tickets, id)
	return nil
}

// Reserve atomically decrements available seats.
// Status transitions to sold_out when seats reach zero.
func (s *Store) Reserve(id int64, seats int) (*models.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tickets[id]
	if !ok {
		return nil, ErrNotFound
	}
	if t.Status == models.StatusClosed {
		return nil, ErrTicketClosed
	}
	if t.AvailableSeats < seats {
		return nil, ErrNoSeats
	}

	t.AvailableSeats -= seats
	if t.AvailableSeats == 0 {
		t.Status = models.StatusSoldOut
	}
	t.UpdatedAt = time.Now().UTC()
	return clone(t), nil
}

// clone returns a shallow copy so callers cannot mutate the store's internal pointer.
func clone(t *models.Ticket) *models.Ticket {
	c := *t
	return &c
}
