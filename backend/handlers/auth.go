package handlers

import (
	"encoding/json"
	"net/http"

	"weddinghub/models"
)

var Users []models.User

// POST /signup
func Signup(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user models.User

	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user.ID = len(Users) + 1

	Users = append(Users, user)

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]any{
		"message": "User created successfully",
		"user":    user,
	})
}

// GET /users
func GetUsers(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(Users)
}

// POST /login
func Login(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var login struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&login)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	for _, user := range Users {

		if user.Email == login.Email &&
			user.Password == login.Password {

			w.Header().Set("Content-Type", "application/json")

			json.NewEncoder(w).Encode(map[string]string{
				"message": "Login successful",
			})

			return
		}
	}

	http.Error(w, "Invalid email or password", http.StatusUnauthorized)
}
