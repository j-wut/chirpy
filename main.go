package main

import (
	"io"
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiState struct {
	fileserverHits atomic.Int32
}

func (state *apiState) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (state *apiState) hitsHandler(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
	io.WriteString(w, fmt.Sprintf(
`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
  </html>`, state.fileserverHits.Load()))
}

func (state *apiState) resetHitsHandler(w http.ResponseWriter, request *http.Request) {
	state.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	io.WriteString(w, "OK")

}


func readiness(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	io.WriteString(w, "OK")
}

func main() {

	metrics := &apiState{
		fileserverHits: atomic.Int32{},
	}

	metrics.fileserverHits.Store(0)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}
	mux.Handle("GET /app/", http.StripPrefix("/app", metrics.middlewareMetricsInc(http.FileServer(http.Dir("./site")))))
	mux.HandleFunc("GET /api/healthz", readiness)

	mux.HandleFunc("GET /admin/metrics", metrics.hitsHandler)
	mux.HandleFunc("POST /admin/reset", metrics.resetHitsHandler)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println(err) 
	}
}
