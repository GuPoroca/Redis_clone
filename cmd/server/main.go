package main

import (
	"flag"
	"log"

	"redis-clone/internal/server"
)

func main() {
	addr := flag.String("addr", ":6379", "TCP address to listen on")
	flag.Parse()

	srv := server.New(*addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
