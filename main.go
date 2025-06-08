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
	"time"
	"strings"

	"github.com/joho/godotenv"
	"github.com/google/uuid"

	"github.com/j-wut/chirpy/internal/database"
)

type errorResponse struct {
	Error string `json:"error"`
}

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) hitsHandler(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
	io.WriteString(w, fmt.Sprintf(
`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
  </html>`, cfg.fileserverHits.Load()))
}

func (cfg *apiConfig) resetHitsHandler(w http.ResponseWriter, request *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	io.WriteString(w, "OK")

}

func readiness(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	io.WriteString(w, "OK")
}

func (cfg *apiConfig) resetUsers(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")

	if strings.ToLower(platform) != "dev" {
		fmt.Printf("WARNING: cannot delete users on %s\n", platform)
		w.WriteHeader(403)
		return
	}

	if err := cfg.dbQueries.ResetUsers(r.Context()); err != nil {
		fmt.Printf("Error resetting users: %s", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(200)
	return

}

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	type createUserRequest struct {
		Email string `json:"email"`
	}
	type createUserResponse struct {
		ID uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	requestBody := createUserRequest{}
	w.Header().Set("Content-Type", "application/json")

	if err := decoder.Decode(&requestBody); err != nil {
		fmt.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		resBody := errorResponse{
			Error: fmt.Sprintf("%s", err),
		}
		resStr, _ := json.Marshal(resBody)
		w.Write(resStr)
		return
	}

	user, err := cfg.dbQueries.CreateUser(r.Context(), requestBody.Email)
	if err != nil {
		fmt.Printf("Error creating user: %s", err)
		w.WriteHeader(500)
		resBody := errorResponse{
			Error: fmt.Sprintf("%s", err),
		}
		resStr, _ := json.Marshal(resBody)
		w.Write(resStr)
		return
	}

	resBody := createUserResponse{
		ID: user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email: user.Email,
	}
	resStr, err := json.Marshal(resBody)
	if err != nil {
		fmt.Printf("Error Marshalling new user: %s", err)
		w.WriteHeader(500)
		resBody := errorResponse{
			Error: fmt.Sprintf("%s", err),
		}
		resStr, _ := json.Marshal(resBody)
		w.Write(resStr)
		return
	}

	w.WriteHeader(201)
	w.Write(resStr)
	return
}

func (cfg *apiConfig) createChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	type createChirpResponse struct {
		ID uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body string `json:"body"`
		UserID uuid.UUID `json:"user_id"`
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
	

	chirp, err := cfg.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{Body: cleaned, UserID:params.UserID})
	if err != nil {
		w.WriteHeader(500)
		resBody := errorResponse{
			Error: fmt.Sprintf("%s", err),
		}
		resStr, _ := json.Marshal(resBody)
		w.Write(resStr)
		return
	}

	
	w.WriteHeader(201)
	resStr, _ := json.Marshal(createChirpResponse(chirp))
	w.Write(resStr)
	return
}

func main() {

	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(fmt.Errorf("Error Connecting to DB: %s", err))
	}
	fmt.Printf("Connected to: %s\n", dbURL)

	dbQueries := database.New(db)

	metrics := &apiConfig{
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
	mux.HandleFunc("POST /admin/reset", metrics.resetUsers)

	mux.HandleFunc("POST /api/chirps", metrics.createChirp)
	mux.HandleFunc("POST /api/users", metrics.createUser)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println(err) 
	}
}
