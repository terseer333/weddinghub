package main

import (
	"fmt"
	"net/http"

	"weddinghub/handlers"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "WeddingHub API Running 🚀")
}

func main() {
	http.HandleFunc("/", home)

	http.HandleFunc("/signup", handlers.Signup)
	http.HandleFunc("/users", handlers.GetUsers)
	http.HandleFunc("/login", handlers.Login)

	http.HandleFunc("/upload", handlers.UploadPhoto)
	http.HandleFunc("/events", handlers.CreateEvent)
	http.HandleFunc("/events/all", handlers.GetEvents)
	http.HandleFunc("/event", handlers.GetEvent)
	http.HandleFunc("/events/delete", handlers.DeleteEvent)

	http.HandleFunc("/rsvp", handlers.SubmitRSVP)
	http.HandleFunc("/rsvp/all", handlers.GetRSVPs)
	http.HandleFunc("/chat", handlers.GuestChat)

	fs := http.FileServer(
		http.Dir("./uploads"),
	)

	http.Handle(
		"/uploads/",
		http.StripPrefix(
			"/uploads/",
			fs,
		),
	)

	fmt.Println("Server running on http://localhost:8080")

	err := http.ListenAndServe(
		":8080",
		enableCORS(http.DefaultServeMux),
	)
	if err != nil {
		fmt.Println(err)
	}
}
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")

		if r.Method == "OPTIONS" {
			return
		}

		next.ServeHTTP(w, r)
	})
}
