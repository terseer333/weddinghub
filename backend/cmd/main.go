package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"weddinghub/api"
	"weddinghub/repository"
)

func main() {
	address := os.Getenv("WEDDINGHUB_ADDR")
	if address == "" {
		address = ":8080"
	}

	server := &http.Server{
		Addr:              address,
		Handler:           api.New(repository.NewMemoryRepository()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("WeddingHub MVP API listening on %s", address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
