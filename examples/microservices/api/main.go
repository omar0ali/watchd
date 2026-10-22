// api is a tiny JSON service used by the watchd example.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	started := time.Now()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	http.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"message": "hello from api",
			"env":     os.Getenv("APP_ENV"),
			"uptime":  time.Since(started).Round(time.Second).String(),
		})
	})

	log.Printf("api listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
