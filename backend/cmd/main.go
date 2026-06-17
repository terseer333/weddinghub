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

	fmt.Println("Server running on :8080")

	http.ListenAndServe(":8080", nil)
	http.HandleFunc("/users", handlers.GetUsers)
}

