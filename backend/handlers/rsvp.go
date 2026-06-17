package handlers

import (
	"encoding/json"
	"net/http"

	"weddinghub/models"
)

var RSVPs []models.RSVP

func SubmitRSVP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rsvp models.RSVP

	err := json.NewDecoder(r.Body).Decode(&rsvp)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	rsvp.ID = len(RSVPs) + 1

	RSVPs = append(RSVPs, rsvp)

	json.NewEncoder(w).Encode(rsvp)
}
