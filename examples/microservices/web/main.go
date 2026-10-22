// web is a tiny frontend that calls the api, used by the watchd example.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	apiURL := os.Getenv("API_URL")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get(apiURL + "/api/hello")
		if err != nil {
			http.Error(w, "api unavailable: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(w, "<h1>web</h1><p>api says: <code>%s</code></p>", body)
	})

	log.Printf("web listening on :%s (api at %s)", port, apiURL)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
