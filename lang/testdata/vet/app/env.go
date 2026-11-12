package app

import (
	"log"
	"os"
)

func host() string {
	if os.Getenv("HOST") == "" {
		log.Fatal("no host")
	}
	return os.Getenv("HOST")
}
