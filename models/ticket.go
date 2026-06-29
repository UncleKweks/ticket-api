package models

import "time"

// Status represents the lifecycle state of a ticket listing.
type Status string

const (
	StatusOpen    Status = "open"
	StatusClosed  Status = "closed"
	StatusSoldOut Status = "sold_out"
)

// Ticket is the core domain object.
type Ticket struct {
	ID             int64     `json:"id"`
	Event          string    `json:"event"`
	Description    string    `json:"description"`
	Price          float64   `json:"price"`
	TotalSeats     int       `json:"total_seats"`
	AvailableSeats int       `json:"available_seats"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateRequest is the payload for creating a new ticket listing.
type CreateRequest struct {
	Event       string  `json:"event"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	TotalSeats  int     `json:"total_seats"`
}

// UpdateRequest uses pointer fields so that a missing JSON key is treated as
// "no change" rather than "set to zero value". This is the standard Go pattern
// for partial (PATCH-style) updates sent via PUT.
type UpdateRequest struct {
	Event       *string  `json:"event"`
	Description *string  `json:"description"`
	Price       *float64 `json:"price"`
	Status      *Status  `json:"status"`
}

// ReserveRequest is the payload for reserving seats on a ticket.
type ReserveRequest struct {
	Seats int `json:"seats"`
}
