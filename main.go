package main

import _ "github.com/lib/pq"

import (
	"io"
	"fmt"
	"net/http"
	"sync/atomic"
	"encoding/json"
	"regexp"
	"os"
	"database/sql"

	"github.com/j-wut/chirpy/internal/database"
)

type apiState struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
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

func validateChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	type validResponse struct {
		CleanedBody string `json:"cleaned_body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	w.Header().Set("Content-Type", "application/json")

	if err := decoder.Decode(&params); err != nil {
		fmt.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		resBody := errorResponse{
			Error: fmt.Sprintf("%s", err),
		}
		resStr, _ := json.Marshal(resBody)
		w.Write(resStr)
		return
	}

	if len(params.Body) > 140 {
		w.WriteHeader(400)
		resBody := errorResponse{
			Error: "Chirp is too long",
		}
		resStr, _ := json.Marshal(resBody)
		w.Write(resStr)
		return
	}
	profane := []string{"kerfuffle", "sharbert", "fornax"}

	cleaned := params.Body

	for _, s := range(profane) {
		re := regexp.MustCompile(`(?i)`+s)
		cleaned = re.ReplaceAllString(cleaned, "****")
	}
		
	
	w.WriteHeader(200)
	resBody := validResponse{
		CleanedBody: cleaned,
	}
	resStr, _ := json.Marshal(resBody)
	w.Write(resStr)
	return
}

func main() {

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(fmt.Errorf("Error Connecting to DB: %s", err))
	}

	dbQueries := database.New(db)

	metrics := &apiState{
		fileserverHits: atomic.Int32{},
		dbQueries: dbQueries,
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

	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println(err) 
	}
}
