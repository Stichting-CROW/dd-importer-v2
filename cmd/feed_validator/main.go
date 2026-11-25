// Command feed_validator is an internal HTTP service that checks an operator
// feed against the data standard. The admin API calls it; it is not exposed
// outside the cluster and has no database access.
package main

import (
	"context"
	"crypto/subtle"
	"deelfietsdashboard-importer/validator"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	tokenHeader       = "X-Feed-Validator-Token"
	maxRequestBytes   = 1 << 20
	validationTimeout = 55 * time.Second
)

func main() {
	token := os.Getenv("FEED_VALIDATOR_TOKEN")
	if token == "" {
		log.Fatal("FEED_VALIDATOR_TOKEN must be set")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.Handle("POST /validate", validateHandler(token))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      validationTimeout + 5*time.Second,
	}
	log.Printf("Feed validator listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func validateHandler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(tokenHeader)), []byte(token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
			return
		}

		var request validator.Request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), validationTimeout)
		defer cancel()
		result := validator.Validate(ctx, request)

		// Feed URLs and credentials are never logged.
		log.Printf("validated system_id=%s data_type=%s feed_type=%s status=%s files=%d duration=%dms",
			request.SystemID, request.DataType, request.FeedType, result.Status, len(result.Files), result.DurationMs)
		writeJSON(w, http.StatusOK, result)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
