package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	var password string

	flag.StringVar(&password, "password", "", "admin password to hash")
	flag.Parse()

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(os.Stdout, "Update the admin user in postgres with this password_hash:\n%q\n", hashed)
}
