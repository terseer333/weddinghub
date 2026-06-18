package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

	user.Email = strings.TrimSpace(user.Email)
	user.Password = strings.TrimSpace(user.Password)

	user.ID = len(Users) + 1

	Users = append(Users, user)

	fmt.Println("User Registered:")
	fmt.Printf("Email: '%s'\n", user.Email)
	fmt.Printf("Password: '%s'\n", user.Password)

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

	login.Email = strings.TrimSpace(login.Email)
	login.Password = strings.TrimSpace(login.Password)

	fmt.Println("\n========== LOGIN ATTEMPT ==========")
	fmt.Printf("Entered Email: '%s'\n", login.Email)
	fmt.Printf("Entered Password: '%s'\n", login.Password)

	for _, user := range Users {

		fmt.Println("Checking against stored user:")
		fmt.Printf("Stored Email: '%s'\n", user.Email)
		fmt.Printf("Stored Password: '%s'\n", user.Password)

		if user.Email == login.Email &&
			user.Password == login.Password {

			fmt.Println("✅ LOGIN SUCCESSFUL")

			w.Header().Set("Content-Type", "application/json")

			json.NewEncoder(w).Encode(map[string]string{
				"message": "Login successful",
			})

			return
		}
	}

	fmt.Println("❌ LOGIN FAILED")

	http.Error(w, "Invalid email or password", http.StatusUnauthorized)
}
