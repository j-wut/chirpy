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
  "github.com/j-wut/chirpy/internal/auth"
)

type UserRequest struct {
	Email             string  `json:"email"`
  Password          string  `json:"password"`
  ExpiresInSeconds  uint    `json:"expires_in_seconds"`
}

type ReadableUser struct {
  ID            uuid.UUID `json:"id"`
  CreatedAt     time.Time `json:"created_at"`
  UpdatedAt     time.Time `json:"updated_at"`
  Email         string    `json:"email"`
  Token         string    `json:"token,omitempty"` 
  RefreshToken  string    `json:"refresh_token,omitempty"`
}

func DatabaseUserToReadable(user database.User) ReadableUser {
  return ReadableUser{
    user.ID,
    user.CreatedAt,
    user.UpdatedAt,
    user.Email,
    "",
    "",
  }
}

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
  jwtSecret string
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
	decoder := json.NewDecoder(r.Body)
  requestBody := UserRequest{}

	if err := decoder.Decode(&requestBody); err != nil {
		fmt.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

  hashedPass, err := auth.HashPassword(requestBody.Password)
  if err != nil {
    fmt.Printf("Error hashing password: %s", err)
    w.WriteHeader(500)
    return
  }

	user, err := cfg.dbQueries.CreateUser(r.Context(), database.CreateUserParams{requestBody.Email, hashedPass})
	if err != nil {
		fmt.Printf("Error creating user: %s", err)
		w.WriteHeader(500)
		return
	}

	resStr, err := json.Marshal(DatabaseUserToReadable(user))
	if err != nil {
		fmt.Printf("Error Marshalling user: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(resStr)
	return
}

func (cfg *apiConfig) login(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	requestBody := UserRequest{}

	if err := decoder.Decode(&requestBody); err != nil {
		fmt.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	user, err := cfg.dbQueries.GetUser(r.Context(), requestBody.Email)
	if err != nil {
		fmt.Printf("Error retrieving user: %s", err)
		w.WriteHeader(401)
    w.Write([]byte("Incorrect email or password"))
		return
	}
  
  err = auth.CheckPasswordHash(requestBody.Password, user.HashedPassword)
  if err != nil {
    fmt.Println("Invalid Password")
    w.WriteHeader(401)
    w.Write([]byte("Incorrect email or password"))
    return
  }
  
  readableUser := DatabaseUserToReadable(user)

  var tokenDuration time.Duration
  if requestBody.ExpiresInSeconds == 0 || time.Second * time.Duration(requestBody.ExpiresInSeconds) > time.Hour {
    tokenDuration = time.Hour
  } else {
    tokenDuration = time.Second * time.Duration(requestBody.ExpiresInSeconds)
  }

  jwtToken, err := auth.MakeJWT(user.ID, cfg.jwtSecret, tokenDuration)
  if err != nil {
    fmt.Printf("Error generating JWT: %s", err)
    w.WriteHeader(500)
    return
  }

  readableUser.Token = jwtToken

  refreshToken, err := auth.MakeRefreshToken()
  if err != nil {
    fmt.Printf("Error generating refresh token: %s", err)
    w.WriteHeader(500)
    return
  }

  _, err = cfg.dbQueries.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{refreshToken, time.Now().AddDate(0,0,60), readableUser.ID})
  if err != nil {
    fmt.Printf("Error saving refresh token: %s", err)
    w.WriteHeader(500)
    return
  }

  readableUser.RefreshToken = refreshToken

  resStr, err := json.Marshal(readableUser)
	if err != nil {
		fmt.Printf("Error Marshalling user: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(resStr)
	return
}

func (cfg *apiConfig) refresh(w http.ResponseWriter, r *http.Request) {
  bearer, err := auth.GetBearerToken(r.Header)
  if err != nil {
    fmt.Println("Invalid refresh token")
    w.WriteHeader(401)
    return
  }

  refreshToken, err := cfg.dbQueries.GetRefreshToken(r.Context(), bearer)
  if err != nil {
    fmt.Println("Invalid refresh token")
    w.WriteHeader(401)
    return
  }

  jwtToken, err := auth.MakeJWT(refreshToken.UserID, cfg.jwtSecret, time.Hour)
  if err != nil {
    fmt.Printf("Error generating JWT: %s", err)
    w.WriteHeader(500)
    return
  }
  
  type TokenResponse struct {
    Token string `json:"token"`
  }
  res := TokenResponse{jwtToken}
  resStr, err := json.Marshal(res)
	if err != nil {
		fmt.Printf("Error Marshalling Token: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(resStr)
	return
}

func (cfg *apiConfig) revoke(w http.ResponseWriter, r *http.Request) {
  bearer, err := auth.GetBearerToken(r.Header)
  if err != nil {
    fmt.Println("Invalid refresh token")
    w.WriteHeader(401)
    return
  }

  err = cfg.dbQueries.RevokeRefreshToken(r.Context(), bearer)
  if err != nil {
    fmt.Println("Invalid refresh token")
    w.WriteHeader(401)
    return
  }

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(204)
	return
}

func (cfg *apiConfig) createChirp(w http.ResponseWriter, r *http.Request) {
  bearer, err := auth.GetBearerToken(r.Header)
  if err != nil {
    w.WriteHeader(401)
    return
  }

  userID, err := auth.ValidateJWT(bearer, cfg.jwtSecret)
  if err != nil {
    w.WriteHeader(401)
    return
  }

  decoder := json.NewDecoder(r.Body)
  params := database.CreateChirpParams{
    UserID: userID,
  }

	if err = decoder.Decode(&params); err != nil {
		fmt.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(params.Body) > 140 {
		w.WriteHeader(400)
		w.Write([]byte("Chirp is too long"))
		return
	}
	profane := []string{"kerfuffle", "sharbert", "fornax"}
	for _, s := range(profane) {
		re := regexp.MustCompile(`(?i)`+s)
		params.Body = re.ReplaceAllString(params.Body, "****")
	}
	

	chirp, err := cfg.dbQueries.CreateChirp(r.Context(), params) 
	if err != nil {
    fmt.Printf("Error saving chirp: %s", err)
		w.WriteHeader(500)
		return
	}

	
	w.WriteHeader(201)
	w.Header().Set("Content-Type", "application/json")
	resStr, _ := json.Marshal(chirp)
	w.Write(resStr)
	return
}

func (cfg *apiConfig) getAllChirps(w http.ResponseWriter, r *http.Request) {
	type chirp struct {
		ID        uuid.UUID	`json:"id"`
		CreatedAt time.Time	`json:"created_at"`
		UpdatedAt time.Time	`json:"updated_at"`
		Body      string	`json:"body"`
		UserID    uuid.UUID	`json:"user_id"`
	}


	chirps, err := cfg.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}	

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	resStr, _ := json.Marshal(chirps)
	w.Write(resStr)
	return
}

func (cfg *apiConfig) getChirp(w http.ResponseWriter, r *http.Request) {
	requestedId, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}
	chirp, err := cfg.dbQueries.GetChirp(r.Context(), requestedId)
	if err != nil {
		w.WriteHeader(500)
    w.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}

	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	resStr, _ := json.Marshal(chirp)
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
    jwtSecret: os.Getenv("JWT_SECRET"),
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
	mux.HandleFunc("GET /api/chirps", metrics.getAllChirps)
	mux.HandleFunc("GET /api/chirps/{id}", metrics.getChirp)
	mux.HandleFunc("POST /api/users", metrics.createUser)
  mux.HandleFunc("POST /api/login", metrics.login)
  mux.HandleFunc("POST /api/refresh", metrics.refresh)
  mux.HandleFunc("POST /api/revoke", metrics.revoke)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println(err) 
	}
}
