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

	http.HandleFunc("/events", handlers.CreateEvent)
	http.HandleFunc("/events/all", handlers.GetEvents)

	fmt.Println("Server running on http://localhost:8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
	}
}
