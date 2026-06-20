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
func GetRSVPs(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if RSVPs == nil {
		json.NewEncoder(w).Encode([]models.RSVP{})
		return
	}

	json.NewEncoder(w).Encode(RSVPs)
}
