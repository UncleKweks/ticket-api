package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"ticket-api/models"
	"ticket-api/store"
)

// TicketHandler holds the dependency on the store, making it easy to swap the
// implementation (e.g. a real database) without changing any handler code.
type TicketHandler struct {
	store *store.Store
}

func NewTicketHandler(s *store.Store) *TicketHandler {
	return &TicketHandler{store: s}
}

// List godoc
// GET /api/v1/tickets?status=open
func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	status := models.Status(r.URL.Query().Get("status"))
	tickets := h.store.GetAll(status)
	respond(w, http.StatusOK, tickets)
}

// Get godoc
// GET /api/v1/tickets/{id}
func (h *TicketHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	t, err := h.store.GetByID(id)
	if errors.Is(err, store.ErrNotFound) {
		respondError(w, http.StatusNotFound, "ticket not found")
		return
	}
	respond(w, http.StatusOK, t)
}

// Create godoc
// POST /api/v1/tickets
func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateCreate(req); err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	t := h.store.Create(req)
	respond(w, http.StatusCreated, t)
}

// Update godoc
// PUT /api/v1/tickets/{id}
func (h *TicketHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	var req models.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	t, err := h.store.Update(id, req)
	if errors.Is(err, store.ErrNotFound) {
		respondError(w, http.StatusNotFound, "ticket not found")
		return
	}
	respond(w, http.StatusOK, t)
}

// Delete godoc
// DELETE /api/v1/tickets/{id}
func (h *TicketHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	if err := h.store.Delete(id); errors.Is(err, store.ErrNotFound) {
		respondError(w, http.StatusNotFound, "ticket not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reserve godoc
// POST /api/v1/tickets/{id}/reserve
func (h *TicketHandler) Reserve(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}
	var req models.ReserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Seats <= 0 {
		respondError(w, http.StatusUnprocessableEntity, "seats must be greater than zero")
		return
	}

	t, err := h.store.Reserve(id, req.Seats)
	switch {
	case errors.Is(err, store.ErrNotFound):
		respondError(w, http.StatusNotFound, "ticket not found")
	case errors.Is(err, store.ErrNoSeats):
		respondError(w, http.StatusConflict, "not enough seats available")
	case errors.Is(err, store.ErrTicketClosed):
		respondError(w, http.StatusConflict, "ticket is closed")
	case err != nil:
		respondError(w, http.StatusInternalServerError, "internal server error")
	default:
		respond(w, http.StatusOK, t)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}

func validateCreate(req models.CreateRequest) error {
	if req.Event == "" {
		return errors.New("event is required")
	}
	if req.TotalSeats <= 0 {
		return errors.New("total_seats must be greater than zero")
	}
	if req.Price < 0 {
		return errors.New("price cannot be negative")
	}
	return nil
}
