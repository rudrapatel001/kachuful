package main

import (
	"log"
	"net/http"
	"os"

	"backend/internal/network"
)

func main() {
	roomManager := network.NewRoomManager()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		network.ServeWS(roomManager, w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Kachuful server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
