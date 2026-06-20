package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"weddinghub/models"
)

var Events []models.Event

func CreateEvent(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event models.Event

	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	event.ID = len(Events) + 1

	Events = append(Events, event)

	json.NewEncoder(w).Encode(event)
}

func GetEvents(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(Events)
}
func GetEvent(w http.ResponseWriter, r *http.Request) {

	id := r.URL.Query().Get("id")

	for _, event := range Events {

		if fmt.Sprintf("%d", event.ID) == id {

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(event)
			return
		}
	}

	http.Error(w, "Event not found", http.StatusNotFound)
}
func DeleteEvent(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodDelete {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	id := r.URL.Query().Get("id")

	if id == "" {
		http.Error(
			w,
			"Missing event id",
			http.StatusBadRequest,
		)
		return
	}

	for i, event := range Events {

		if fmt.Sprintf("%d", event.ID) == id {

			Events = append(
				Events[:i],
				Events[i+1:]...,
			)

			json.NewEncoder(w).Encode(
				map[string]string{
					"message": "Event deleted",
				},
			)

			return
		}
	}

	http.Error(
		w,
		"Event not found",
		http.StatusNotFound,
	)
}
