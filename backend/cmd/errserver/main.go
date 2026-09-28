package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	msgFile := flag.String("msg-file", "", "file containing the error message to serve")
	flag.Parse()

	msg, err := os.ReadFile(*msgFile)
	if err != nil {
		log.Fatalf("Failed to read message file: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": string(msg)})
	})

	log.Printf("Serving compilation error on :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
