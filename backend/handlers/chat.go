package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"weddinghub/models"
)

var ChatMessages []models.ChatMessage

func GuestChat(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case http.MethodGet:
		getChatMessages(w, r)
	case http.MethodPost:
		submitChatMessage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getChatMessages(w http.ResponseWriter, r *http.Request) {

	eventID, err := strconv.Atoi(r.URL.Query().Get("event_id"))
	if err != nil || eventID <= 0 {
		http.Error(w, "Missing event id", http.StatusBadRequest)
		return
	}

	rsvpID, err := strconv.Atoi(r.URL.Query().Get("rsvp_id"))
	if err != nil || !isAcceptedGuest(eventID, rsvpID) {
		http.Error(w, "Chat room is only available to attending guests", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	messages := []models.ChatMessage{}
	for _, message := range ChatMessages {
		if message.EventID == eventID {
			messages = append(messages, message)
		}
	}

	json.NewEncoder(w).Encode(messages)
}

func submitChatMessage(w http.ResponseWriter, r *http.Request) {

	var message models.ChatMessage

	err := json.NewDecoder(r.Body).Decode(&message)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !isAcceptedGuest(message.EventID, message.RSVPID) {
		http.Error(w, "Chat room is only available to attending guests", http.StatusForbidden)
		return
	}

	message.Message = strings.TrimSpace(message.Message)
	if message.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	message.ID = len(ChatMessages) + 1
	message.CreatedAt = time.Now().Format(time.RFC3339)
	message.GuestName = acceptedGuestName(message.EventID, message.RSVPID)

	ChatMessages = append(ChatMessages, message)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(message)
}

func isAcceptedGuest(eventID int, rsvpID int) bool {

	for _, rsvp := range RSVPs {
		if rsvp.ID == rsvpID &&
			rsvp.EventID == eventID &&
			strings.EqualFold(rsvp.Status, "attending") {
			return true
		}
	}

	return false
}

func acceptedGuestName(eventID int, rsvpID int) string {

	for _, rsvp := range RSVPs {
		if rsvp.ID == rsvpID && rsvp.EventID == eventID {
			return rsvp.GuestName
		}
	}

	return fmt.Sprintf("Guest %d", rsvpID)
}
