package main

import (
	"io"
	"fmt"
	"net/http"
)

func readiness(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	io.WriteString(w, "OK")
}

func main() {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}
	mux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("./site"))))
	mux.HandleFunc("/healthz", readiness)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println(err) 
	}
}
