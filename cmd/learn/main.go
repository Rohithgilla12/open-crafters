package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Rohithgilla12/open-crafters/internal/learn"
)

func main() {
	listen := os.Getenv("LEARN_LISTEN")
	if listen == "" {
		listen = ":8081"
	}

	catalog, err := learn.NewCatalog()
	if err != nil {
		log.Fatalf("learn: %v", err)
	}

	srv := learn.NewServer(catalog)
	log.Printf("learn: listening on %s (%d challenges)", listen, len(catalog.Order))
	if err := http.ListenAndServe(listen, srv.Handler()); err != nil {
		log.Fatalf("learn: %v", err)
	}
}
