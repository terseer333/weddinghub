package models

type RSVP struct {
	ID         int    `json:"id"`
	EventID    int    `json:"event_id"`
	GuestName  string `json:"guest_name"`
	Phone      string `json:"phone"`
	Status     string `json:"status"`
	GuestCount int    `json:"guest_count"`
	Message    string `json:"message"`
}
