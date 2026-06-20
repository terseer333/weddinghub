package models

type ChatMessage struct {
	ID        int    `json:"id"`
	EventID   int    `json:"event_id"`
	RSVPID    int    `json:"rsvp_id"`
	GuestName string `json:"guest_name"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}
