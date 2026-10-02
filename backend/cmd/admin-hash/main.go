package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password, err := io.ReadAll(io.LimitReader(os.Stdin, 74))
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not read password")
		os.Exit(1)
	}
	if len(password) < 10 || len(password) > 72 {
		fmt.Fprintln(os.Stderr, "password must be between 10 and 72 bytes")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not hash password")
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
